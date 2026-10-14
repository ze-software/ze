# Spec: bgp-prefix-sid-rfc-defects

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/5 |
| Handoff | - |
| Updated | 2026-10-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The strict RFC re-read of `spec-rfc-requirement-quote-hand-backfill`
found five places where Ze's handling of a received BGP Prefix-SID attribute
(RFC 8669) and its SRv6 Service TLVs (RFC 9252) does not do what the RFC
requires. An operator meets each one on the wire: a malformed attribute is
propagated, a malformed SRv6 Service TLV gets the wrong RFC 7606 action, and a
path with no valid SRv6 SID stays a best-path candidate.

Each row below was confirmed open at its producer in HEAD on 2026-09-27 and
quoted from `rfc/full/`. The fix makes Ze do what the quoted sentence says.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 8669 Section 6 (RFC8669-6-1) | "a malformed BGP Prefix-SID attribute is one that cannot be parsed due to not meeting the minimum attribute length requirement, containing a TLV length that doesn't conform to the length constraints for the TLV, or containing a TLV length that would extend beyond the end of the attribute" | `internal/component/bgp/message/rfc7606.go::validatePrefixSIDAttr` | Only the overrun and trailing-byte cases are checked. A Label-Index TLV whose length is not 7, an Originator SRGB TLV whose length is not 2 plus a non-zero multiple of 6, and a zero-length attribute all pass and are propagated | weak |
| D2 | RFC 9252 Section 7 (RFC9252-3.4-1) | "The treat-as-withdraw action [RFC7606] MUST be performed when at least one malformed SRv6 Service TLV is present in the BGP Prefix-SID attribute." | `internal/component/bgp/message/rfc7606.go::validatePrefixSIDAttr` | A type 5 or 6 TLV whose length overruns the attribute returns attribute-discard before the type is read, where the TLV is a malformed SRv6 Service TLV ("The TLV Length is inconsistent with the length of the BGP Prefix-SID attribute") and owes treat-as-withdraw | weak |
| D3 | RFC 9252 Section 7 (RFC9252-3.2.1-3) | "The SRv6 SID value in the SRv6 SID Information Sub-TLV is invalid when the SID Structure Sub-Sub-TLV transposition length is greater than the number of bits of the label field or if any of the conditions for the fields of the Sub-Sub-TLV, as specified in Section 3.2.1, is not met." | `internal/component/bgp/plugins/rib/pool/srv6sid.go::parseSIDStructure`, read by `extractSIDFromServiceTLV` | An invalid SID Structure only clears the transposition fields; the SID itself is still returned as valid and installed | weak |
| D4 | RFC 9252 Section 7 (RFC9252-5-1) | "The path having any such Prefix-SID attribute without any valid SRv6 SID information MUST be considered ineligible during the selection of the best path for the corresponding prefix." | `internal/component/bgp/plugins/rib/rib_bestchange.go::isSRv6Ineligible` | The candidate filter decides on `pool.ExtractSRv6SID`, which answers valid for an invalid SID Structure (D3) and never checks the transposition length against the label width. That bound is applied only later in `srv6SIDFromResult`, after selection, so such a path still wins | weak |
| D5 | RFC 9252 Section 7 (RFC9252-7-1) | "If multiple instances of the SRv6 L3 Service TLV are encountered, all but the first instance MUST be ignored." | `internal/component/bgp/plugins/rib/pool/srv6sid.go::ExtractSRv6SIDFull` | The loop returns the first Service TLV that yields a valid SID, so when the first L3 (or L2) instance carries none, the second instance is used instead of ignored | weak |
| D6 | RFC 8669 Section 6 (RFC8669-6-3), RFC 9252 Section 7 | "Similarly, if a recognized TLV appears more than once in a BGP Prefix-SID attribute while the specification only allows for a single occurrence, then all the occurrences of the TLV other than the first one SHALL be discarded and the Prefix-SID attribute will continue to be processed." | Received UPDATE normalization and subsequent Prefix-SID forwarding (`reactor/rfc8669_duplicate_tlv.go::discardRepeatedPrefixSIDTLVs`); the red probe was removed once it turned green in `a41bf6a186` | First-wins extraction alone leaves later recognized single-occurrence TLVs in the relayed attribute. Types 5 and 6 are single-occurrence under RFC 9252 Section 7. | weak; owner transferred this defect here under P-3 on 2026-10-02 |

### Status at HEAD, re-checked 2026-10-13

Each row re-read at its producer on 2026-10-13 (HEAD `70eb75e6df`), against
`rfc/audit/rfc8669.json`, `rfc/audit/rfc9252.json`, the rows of
`plan/pre-release/spec-rfc-verdict-fix-bgp.md` that name this spec, and a `./le rfc check` run (OK, 4147 gated requirements; it
reports nothing against the rows below).

| ID | Open at HEAD? | Evidence at the producer | Verdict now |
|----|---------------|--------------------------|-------------|
| D1 | open | `validatePrefixSIDAttr` checks only the TLV overrun and trailing octets; a zero-length attribute leaves its loop at once and returns nil; types 1 and 3 are never length-checked on the receive path | RFC8669-6-1 weak; RFC8669-3.1-2 and RFC8669-3.2-4 weak |
| D2 | open | the overrun test in `validatePrefixSIDAttr` returns attribute-discard before the TLV type is read | RFC9252-3.4-1 weak |
| D3 | open | `parseSIDStructure` answers "no transposition" on a failed sum check; `extractSIDFromServiceTLV` still returns the SID as valid | RFC9252-3.2.1-3 weak |
| D4 | open | `isSRv6Ineligible(entry)` has no family, asks `pool.ExtractSRv6SID`, never compares the transposition length with the label width | RFC9252-5-1 weak |
| D5 | open | `ExtractSRv6SIDFull` returns the first Service TLV whose SID is valid, so a second instance is used when the first yields none | RFC9252-7-1 and RFC9252-7-2 weak |
| D6 | FIXED | `a41bf6a186` (discard at ingest, red probe `rfc8669_duplicate_tlv_red_test.go` turned green and removed), `465af38f1d` (review hardening), `0c6ae07e69` (interop `bgp-prefix-sid-duplicate-tlv-frr`), `96dbfd4da5` (independent re-judge) | RFC8669-6-3 enforced. AC-8 and AC-9 are met |

Related specs, not duplicated here: `plan/immediate/spec-srv6-bestpath-resolvability.md`
owns RFC9252-5-2 (reachability of the SID) at the same candidate filter, and
`plan/immediate/spec-srv6-evpn-label-width.md` owns EVPN transposition. D4 is the
semantic-validity half that spec names as separate work.

### Sequencing with other specs

| Spec | Conflict | Landing order |
|------|----------|---------------|
| `plan/immediate/spec-bgp-update-propagation-rfc-defects.md` | it folds `clearTransmittedLabelIndex` into its joined `writeUpdateBody` walk with no behavior change; AC-10 here gates that clear to labeled unicast (R-8) | it lands after this spec, or it carries the AC-10 family gate into its walk |
| `plan/immediate/spec-srv6-bestpath-resolvability.md` | its A-6 and "Behavior to preserve" assume `isSRv6Ineligible` and both RFC9252-5-1 tagged tests unchanged; this spec changes all three (R-9) | it lands after this spec and builds on the new predicate |
| `plan/immediate/spec-srv6-evpn-label-width.md` | Q4.2 (a) makes EVPN SIDs with a non-zero AL invalid; Q4.1 names the transposition set that spec extends | no order forced; that spec reads the Q4 answers |

Neither other spec is edited by this one; each conflict is recorded here for its owner.

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | Each row gets a unit test that fails against HEAD before the producer changes |
| Both polarities | A conformant attribute is accepted and propagated unchanged; each malformed shape gets the action the quote names |
| Discrimination | Each RFC-tagged unit carries a record from `./le rfc discriminate-record` (`ai/rules/rfc-compliance.md`) |
| Real entry point | A `.ci` functional test sends the UPDATE from a peer and asserts the RIB and the re-advertisement (D1, D2), and the best path chosen (D4) |
| Interop | An interop scenario against FRR or GoBGP sending SRv6 L3VPN routes, asserting the treat-as-withdraw and ineligibility outcomes (`ai/rules/interop-and-goal-validation.md`) |

## Owner decisions

| Row | Question |
|-----|----------|
| Q1 | RFC8669-6-1 reads "a malformed **or invalid** BGP Prefix-SID attribute", and RFC 8669 Section 4.1 defines invalid: "A BGP Prefix-SID attribute received without a Label-Index TLV MUST be considered to be "invalid" by the receiving speaker." Those are the `{gap}` rows RFC8669-4.1-1 and RFC8669-4.1-2, whose annotation says recording them "does not commission missing SR-MPLS semantics". AC-6 asks RFC8669-6-1 to reach `enforced`, which it cannot while the invalid half is absent. Which way: (a) bring RFC8669-4.1-1/4.1-2 into this spec (attribute-discard on IPv4/IPv6 labeled unicast when no Label-Index TLV is present; AC-1b below), or (b) narrow AC-6 to the malformed half and leave RFC8669-6-1 `weak` with the invalid half named as the remaining gap. Recommended: (a), one more check in the same family-aware step. **Owner, 2026-10-14: accept (recommendation)**: (a), RFC8669-4.1-1/4.1-2 brought into this spec (AC-1b) |
| Q2 | An empty attribute (length 0). RFC 8669 Section 6 binds only "attached to an IPv4/ IPv6 Labeled Unicast prefix", and RFC 9252 Section 7 lists no attribute-level minimum. On labeled unicast it is malformed (AC-1). Off labeled unicast: (a) attribute-discard on every family, as `ParsePrefixSID` already refuses it (`errEmptyPrefixSIDAttribute`), or (b) keep it and propagate. Recommended: (a), because an attribute that `ParsePrefixSID` refuses breaks every reader of the attribute set (`AttributesWire.parseSpan` returns the error). Consequence of (b): `ParsePrefixSID` must also stop refusing an empty value (`errEmptyPrefixSIDAttribute`), or every reader of a kept empty attribute fails, the relay's `packWithContext` included. **Owner, 2026-10-14: accept (recommendation)**: (a), attribute-discard of an empty attribute on every family |
| Q3 | RFC8669-3.1-2 and RFC8669-3.2-4: "It MUST be ignored when received for other BGP AFI/SAFI combinations." The JSON formatter (`appendPrefixSIDJSON`) has no family and renders `sr-label-index` and the SRGB on every family. Which way: (a) leave the JSON rendering as a faithful report of the received bytes and hold "ignored" to mean no validation, no selection and no forwarding effect (AC-7a), or (b) thread the route family into the formatter and render types 1 and 3 off labeled unicast as unknown TLVs. Recommended: (a); (b) changes a published JSON shape. **Owner, 2026-10-14: accept (recommendation)**: (a), the JSON keeps rendering types 1 and 3 on every family (AC-7a) |
| Q4 | RFC 9252 Section 7 makes a SID invalid when "any of the conditions for the fields of the Sub-Sub-TLV, as specified in Section 3.2.1, is not met". Two decisions. **Q4.1, transposition zero rows.** Section 3.2.1 also says "In this case, the Transposition Offset MUST be set to 0" (row RFC9252-3.2.1-1, `{gap}`, no owning spec) and Section 7 says "The transposition offset and length MUST be 0 when the Sub-Sub-TLV is advertised along with routes where the Transposition Scheme is not applicable (e.g., for global IPv6 service [RFC2545] where there is no label field)" (row RFC9252-3.2.1-2, `{gap}`, no owning spec). (a) bring both into AC-3/AC-4, or (b) leave them as gaps. Recommended: (a), same producer, same test file; leaving them out keeps RFC9252-5-1 short of its own definition of invalid. Under (a) the Transposition Scheme applies to IPv4/IPv6 VPN (SAFI 128, 20-bit label, the set `nlrisplit.TranspositionLabel` reads a label from) and to EVPN (24 bits, owned by `plan/immediate/spec-srv6-evpn-label-width.md`); it does not apply to any other family, IPv4/IPv6 unicast and IPv4/IPv6 labeled unicast (SAFI 4) included: RFC 9252 defines no SRv6 service over labeled unicast, although SAFI 4 carries a 20-bit RFC 8277 label. That set is declared once: `labelWidthForSAFI` answers 0 outside it and is derived from the same declaration as `nlrisplit.TranspositionLabel`. **Q4.2, RFC9252-3.2-5.** The sentence (in Section 3.2.1 of `rfc/full/rfc9252.txt`; `rfc/short/rfc9252.md` cites S3.2 for both RFC9252-3.2-5 and RFC9252-3.2-6, while both sentences sit in Section 3.2.1. A stale citation in the summary, noted here and not edited by this spec): "A receiver is unable to validate the applicability of arguments for SRv6 Endpoint Behaviors that are unknown to it and hence MUST ignore SRv6 SIDs with arguments (indicated by a non-zero AL) with unknown SRv6 Endpoint Behaviors." It needs no registry: Ze knows no Endpoint Behavior (nothing under `rib/` or `rib/pool/` reads the behavior code, and `TestExtractSRv6SID_UnknownEndpointBehavior` pins that every code is accepted), so every behavior is unknown to Ze. (a) a SID whose SID Structure carries AL != 0 is invalid (same producer as D3, one more test, `TestExtractSRv6SIDFull_NonZeroArgumentLengthInvalid`); (b) leave RFC9252-3.2-5 a `{gap}`, with its reason corrected to "Ze does not apply it", not "needs a registry". Recommended: (a). Consequence of (a): EVPN SIDs with arguments (End.DT2M, the RFC's own example) become invalid and their paths ineligible, which is the RFC behavior for a receiver that does not know the behavior; `plan/immediate/spec-srv6-evpn-label-width.md` is affected. Consequence of (b): Ze keeps installing SIDs whose arguments it cannot validate. Out either way: RFC9252-3.2-6 binds only a known behavior, and Ze has none; RFC9252-3.2.1-6 ("the AL MUST be set to 0") binds the advertiser, and Ze allocates no local SID (`rfc/short/rfc9252.md`, Support remaining). **Owner, 2026-10-14: accept (recommendation)**: Q4.1 (a), RFC9252-3.2.1-1/3.2.1-2 brought into AC-3/AC-4; Q4.2 (a), a SID with AL != 0 is invalid, so EVPN End.DT2M SIDs with arguments become ineligible (`plan/immediate/spec-srv6-evpn-label-width.md` reads this answer) |
| Q5 | Nine RFC-tagged tests change (TDD Test Plan, "RFC-tagged tests that change"). Design review r1 (2026-10-14) added rows 7 and 8, `TestValidateSRv6ServiceTLV_TrailingBytes` and `TestValidateSRv6ServiceTLV_SIDInfoTooShort` (RFC9252-3.4-1 negative): each asserts a `Description` substring ("trailing bytes", "< 21") that comes from the `message` copy this spec deletes. Applying r1 added row 9, `TestRFC8669LabelIndexTransmissionClearsReservedAndFlags` (RFC8669-3.1-3 and RFC8669-3.1-5): its IPv4 unicast fixture goes red once AC-10 gates the clear, and it moves to its own labeled-unicast helper. The owner's blanket approval below covers them. The first six, as listed on 2026-10-13: `TestExtractSRv6SIDFull_InvalidSIDStructure` and `TestExtractSRv6SIDFull_SumBelowTransposition` (RFC9252-3.2.1-3 negative, flip to no SID), `TestIsSRv6Ineligible_ValidSID` (RFC9252-5-1 positive, family argument), `TestIsSRv6Ineligible_InvalidSID` (RFC9252-5-1 negative tag withdrawn: structural malformation, not a Section 7 invalid SID), `TestSRv6TranspositionWiderThanLabelFieldIsIneligible` (RFC9252-5-1 negative, gains admission assertion), `TestRFC9252FirstServiceTLVWins` (RFC9252-7-1/7-2 negative prose narrowed). Each needs `./le rfc approve unit ... reason "<owner's words>"` before its edit. Approve the list as written? **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** The edits are approved as listed; the units are under "Owner approval commands" in the TDD plan. Q1 to Q4 and Q6 were answered separately, on 2026-10-14 |
| Q6 | Off IPv4/IPv6 labeled unicast, which RFC 7606 action does a Prefix-SID framing error get (a TLV whose length overruns the attribute, or 1 or 2 stray octets after the last TLV) when SRv6 Service TLVs are involved? RFC 7606 Section 2: "Attribute discard: In this approach, the malformed attribute MUST be discarded and the UPDATE message continues to be processed. This approach MUST NOT be used except in the case of an attribute that has no effect on route selection or installation." RFC 8669 Section 6 names attribute-discard only for a Prefix-SID "attached to an IPv4/ IPv6 Labeled Unicast prefix", and RFC 9252 Section 7 says "The SRv6 overlay service requires the Service SID for forwarding." (a) treat-as-withdraw when a type 5 or 6 TLV was read before the framing error, or is the TLV whose length breaks the framing (a type 5/6 TLV that leaves 1 or 2 stray octets is "The TLV Length is inconsistent with the length of the BGP Prefix-SID attribute"); attribute-discard otherwise, because the attribute then holds nothing that affects installation. (b) treat-as-withdraw only for a type 5/6 TLV that overruns (D2 as written), attribute-discard for every other framing error. Recommended: (a). Consequence of (a): an UPDATE with a valid L3 Service TLV followed by a broken unknown TLV withdraws its routes, and escalates to session-reset under RFC 7606 Section 5.2 when it carries no NLRI (R-3). Consequence of (b): that route is installed without its Service SID: once attribute 40 is gone, `isSRv6Ineligible` finds no SRv6 TLV and admits it, which RFC 7606 Section 2 forbids. AC-2 follows the answer. **Owner, 2026-10-14: accept (recommendation)**: (a), treat-as-withdraw when a type 5 or 6 TLV was read before the framing error or is the TLV that breaks it, attribute-discard otherwise |

On 2026-10-02 Thomas assigned D6 and RFC8669-6-3 to this existing spec rather
than expanding the verdict pass. The earlier five-defect scope is extended by
AC-8/AC-9; D1 through D5 remain unchanged.

On 2026-10-03 Thomas authorised fixing D6 without claiming this spec. That fix
is designed and implemented below (AC-8, and AC-9 up to the independent judge).
D1 to D5, AC-1 to AC-7, and AC-9's `./le rfc audit-stamp ... mode rejudge` are
still open under this spec.

On 2026-10-13 the research and design phases ran (status `design`). Q5, the
tagged-test list, was answered by the owner on 2026-10-13. Q1 to Q4 and Q6 were
answered by the owner on 2026-10-14 (Thomas: "LDP, BFD, GR, Prefix-SID,
accept"): every recommendation, (a) each, and the (b) branches are removed
below. The spec moves to `ready` only when the owner approves the design.

## D6 design (implemented 2026-10-03)

An ingest step of `publishBase`, beside `removeRedundantLargeCommunities`:
`discardRepeatedPrefixSIDTLVs` (`internal/component/bgp/reactor/rfc8669_duplicate_tlv.go`)
walks the Prefix-SID TLVs with a stack bitset of the single-occurrence types
already met. When nothing repeats it returns the UPDATE untouched and allocates
nothing. Otherwise it copies the attribute section from the first repeat on,
drops each later TLV of a single-occurrence type, keeps every other TLV in
received order, fixes the attribute length under the peer's header width, and
rebuilds the body with `message.RebuildUpdateBody`. It runs after the RFC 7606
walk, which validates every TLV, so a malformed repeat is still
treat-as-withdraw, and it runs for route-server clients too.

-> Decision: the single-occurrence set is declared once in
`internal/core/bgp/attribute/prefixsid_wire.go::PrefixSIDTLVSingleOccurrence`,
which takes the family. SRv6 L3 Service (5) and SRv6 L2 Service (6) are single
on every family. RFC 9252 Section 7: "If multiple instances of the SRv6 L3
Service TLV are encountered, all but the first instance MUST be ignored." (and
the same for L2).
-> Decision (Thomas, 2026-10-03, "Single only on labelled unicast"): Label-Index
(1) is single only on IPv4 and IPv6 labeled unicast. RFC 8669 Section 3.1: "It
MUST be ignored when received for other BGP AFI/SAFI combinations." On another
family it is not recognized, so every copy is propagated unmodified like an
unknown TLV (Section 6). The walk reads the family from MP_REACH_NLRI, or IPv4
unicast for an UPDATE without one; an UPDATE carrying both is read by its
MP_REACH_NLRI family, since one attribute serves both route sets.
-> Decision: the Originator SRGB TLV (3) is not in the set on any family. RFC
8669 Section 3.2: "The Originator SRGB TLV MUST NOT be changed during the
propagation of the BGP update." RFC 8669 sets it no occurrence limit. (The
earlier rationale quoted the SRGB field repeating, which describes ranges inside
one TLV, not repeated TLVs; replaced in review round 1.)
-> Decision: an unknown TLV is kept however often it repeats. RFC 8669 Section 6:
"For future extensibility, unknown TLVs MUST be ignored and propagated
unmodified."
-> Constraint: D5 (`ExtractSRv6SIDFull` falling back to the second Service TLV)
is out of D6. On received routes the second TLV is now gone before the RIB sees
it, but the extractor's own rule is still D5's to fix.

### D6 Risks & Assumptions

| ID | Assumption | Status | Evidence |
|----|------------|--------|----------|
| A-1 | No consumer reads TLVs inside attribute 40 before `publishBase` | confirmed | `enforceRFC7606` validates, strips and discards, then publishes; the RIB, both relays and JSON read the published bytes |
| A-2 | The RFC 7606 walk validates a repeated SRv6 Service TLV | confirmed | `validatePrefixSIDAttr` loops over every TLV; `TestRFC8669DuplicateMalformedServiceTLVStillWithdrawn` |
| R-1 | A peer splits one SRGB over several Originator SRGB TLVs | accepted | type 3 is kept on every occurrence |

### D6 Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| No allocation without a repeat | `TestDiscardRepeatedPrefixSIDTLVsAllocatesNothingWithoutRepeat` (AllocsPerRun 0, same pointer back) |
| Header width kept | the extended-length case in `TestRFC8669DuplicateServiceTLVDiscardedOnReceive`, whose value ends in octets unlike the ones before it |
| Copy continues after the repeat, tail kept | the unknown-TLV-after and attribute-after cases in the same test |
| Family | `TestPrefixSIDTLVSingleOccurrenceByFamily`, and the IPv4-unicast cases in both receive tests |
| Order kept, unknown kept | the Label-Index case with an unknown TLV between, and `TestRFC8669SingleOccurrencePrefixSIDKeptWhole` |
| Malformed repeat still withdrawn | `TestRFC8669DuplicateMalformedServiceTLVStillWithdrawn` |
| Peer to peer | `test/plugin/prefixsid-duplicate-tlv-relay.ci` |

### D6 review round 1 (on a41bf6a186 + 0c6ae07e69), dispositions

| # | Severity | Finding | Disposition |
|---|----------|---------|-------------|
| 1 | ISSUE | Three walk mutants survived every test: M1 drop the append of TLVs after the first repeat, M2 drop the tail append of attributes after code 40, M3 extended-length header width 4 to 3 | Fixed. New positive cases: L3 twice then an unknown TLV after the repeat; L3 twice with a COMMUNITIES attribute after code 40 (the published tail is compared octet for octet); L3 twice then an unknown TLV under an Extended Length header with that attribute after. Under `go test -overlay`, M1, M2 and M3 each turn `TestRFC8669DuplicateServiceTLVDiscardedOnReceive` red; the fix is green. The `.ci` now discards an SRv6 L3 Service repeat with an unknown TLV after it and COMMUNITIES after code 40. RFC8669-6-3 discrimination records re-recorded (both unit tests and the `.ci`, citation `seq=3`). The audit verdict stamped in b47b4a2f59 is stale against the changed tests and is left for an independent judge |
| 2 | OWNER | Label-Index single on every family | Fixed per the owner's decision above. `PrefixSIDTLVSingleOccurrence(afi, safi, tlvType)`; tests both ways: `TestPrefixSIDTLVSingleOccurrenceByFamily`, the IPv4-unicast Label-Index repeat kept whole, the IPv4-unicast L3 repeat discarded, and the `.ci` second control (Label-Index twice on IPv4 unicast relayed byte for byte). A family-blind mutant and a no-discard mutant each turn the `.ci` red |
| 3 | ISSUE | Type 3 exclusion quoted the SRGB-field repetition, which is about ranges in one TLV | Fixed: RFC 8669 Section 3.2 "MUST NOT be changed during the propagation" quoted in the function, the doc and this spec |
| 4 | ISSUE | `.ci` header claimed it proves the discard for a route-server client | Fixed: sentence deleted; the rs-client paragraph gives only the AS_PATH and NEXT_HOP reasons |
| 5 | ISSUE | `attributes.md` section 40 said "with one exception" | Fixed: a table names the three cases (this discard, `prefixSIDNextHopHandler` on a next-hop change per RFC 9252 Section 2, the Section 8 egress removal through `prefixSIDAllowedTo`) |
| 6 | NOTE | Debug log lacked the peer | Fixed: `discardRepeatedPrefixSIDTLVs` takes the peer address and logs `"peer"`, like the RFC 7606 strips in `session_validation.go` |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/attributes.md` (section 40) - the receive-side treatment of attribute 40
  → Constraint: it already tabulates the three places Ze changes a received Prefix-SID (D6 discard, next-hop-change removal, Section 8 egress removal); the D1 discard and the D2 withdraw join that page in the phase that codes them.
- [ ] `docs/features/srv6.md` (the RFC 8669 and RFC 9252 tables, and the Propagation row) - the public support claims
  → Constraint: the rows "Malformed attribute: attribute-discard", "Malformed Service TLV: treat-as-withdraw", "Path ineligibility (no valid SID)" and "LBL+LNL+FL+AL <= 128 validation" read Partial and name D1 to D4 (the last one already says the SID "is still used whole instead of being invalid"); each flips in the phase that fixes it, never at closure.
  → Constraint: the Propagation row says "Label-Index Reserved and Flags are cleared on every transmission, including relay", and the paragraph on the final session writers says the same. Both become "on IPv4/IPv6 labeled unicast" in the phase that gates `clearTransmittedLabelIndex` (AC-10).
- [ ] `docs/architecture/fib/fib-depth-4-srv6.md` - `// Design:` of `pool/srv6sid.go`
  → Constraint: names the extraction functions; D3 and D5 change what "extracted" means and the page says so.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - `// Design:` of `rib_bestchange.go` (best-path change tracking)
  → Constraint: checked in phase 3 for any statement about SRv6 admission; the admission gate's new family argument and validity predicate are described there if the page describes `candidateAdmitted`, otherwise it is named unaffected with that reason.
- [ ] `docs/architecture/testing/interop.md` - scenario layout, checker registry, vacuity traps
  → Decision: a scenario directory carries only declarative inputs (`ze.conf`, `frr.conf`, `inject.msg`, `inject-args`); assertions are typed Go checkers registered in `internal/le/interoplab/bgp/checkers.go`, as `bgp-prefix-sid-duplicate-tlv-frr` does.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8669.md`, `rfc/short/rfc9252.md`, and `rfc/full/rfc9252.txt` Sections 3.2.1 and 7
  → Constraint (RFC 8669 Section 6, read in `rfc/full/rfc8669.txt`): "When a BGP speaker receives a BGP UPDATE message containing a malformed or invalid BGP Prefix-SID attribute attached to an IPv4/ IPv6 Labeled Unicast prefix ([RFC8277]), it MUST ignore the received BGP Prefix-SID attribute and not advertise it to other BGP peers." The obligation is scoped to labeled unicast; off labeled unicast the attribute-level framing rules stay as they are today.
  → Constraint (RFC 8669 Section 3.1): "Length: 7, the total length in octets of the value portion of the TLV." and "It MUST be ignored when received for other BGP AFI/SAFI combinations." (Section 3.2, Originator SRGB): "Length: The total length in octets of the value portion of the TLV: 2 + (non-zero multiple of 6)." Both TLVs are recognized only on IPv4/IPv6 labeled unicast, matching the owner decision of 2026-10-03 for D6.
  → Constraint (RFC 9252 Section 7): "An SRv6 Service TLV is considered malformed in the following cases: ... The TLV Length is inconsistent with the length of the BGP Prefix-SID attribute." and "The treat-as-withdraw action [RFC7606] MUST be performed when at least one malformed SRv6 Service TLV is present in the BGP Prefix-SID attribute." The type octet decides the action before the length does.
  → Constraint (RFC 9252 Section 7): "Any TLV, Sub-TLV, or Sub-Sub-TLV is not considered malformed because its Type is unrecognized." and "Any TLV, Sub-TLV, or Sub-Sub-TLV is not considered malformed because of failing any semantic validation of its Value field." So an invalid SID Structure makes the SID invalid and the path ineligible; it is never treat-as-withdraw.
  → Constraint (RFC 9252 Section 3.2.1 with verified erratum 7817, `rfc/errata/rfc9252/7817.txt`): the sum of LBL, LNL, FL and AL "MUST be less than or equal to 128 and greater than or equal to the sum of Transposition Offset and Transposition Length." Equality is valid.
  → Constraint (RFC 9252 Section 3.2): "When multiple SRv6 SID Information Sub-TLVs are present, the ingress PE SHOULD use the SRv6 SID from the first instance of the Sub-TLV." The first SID Information Sub-TLV of the first Service TLV of each type is the only one consulted.
  → Constraint (RFC 9252 Section 3.2.1): "the size of the MPLS Label field is 20 bits in [RFC4364] and [RFC8277], and the size is 24 bits in [RFC7432]." `labelWidthForSAFI` holds that fact once.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/rfc7606.go` - `validatePrefixSIDAttr`, `validateSRv6ServiceTLV`, the walk in `ValidateUpdateRFC7606AddPath`, `attrValidatorFn` (signature code, length, data, isIBGP, asn4: no family)
  → Constraint: per-attribute validators run inside one walk and see no family; MP_REACH_NLRI may follow code 40 in the attribute order, so any family-dependent check runs after the walk, where `mpReachNLRI` (a `MPNLRILocation` carrying AFI and SAFI) is already captured for the result, and the walk already records the AS_PATH range the same way.
  → Constraint: a code 40 whose flags conflict is recorded by the walk (`recordError`, then `continue`) before any value check; the post-walk step skips such an attribute, so its verdict is not recorded twice.
  → Constraint: the `message` copy of `validateSRv6ServiceTLV` is NOT rule for rule the same as `attribute.validateSRv6ServiceTLV`. It has no SID Structure length-6 check, and its Sub-Sub-TLV loop inside a SID Information Sub-TLV (`ssOff+3 <= subLen`) has no end check, so 1 or 2 stray octets after the last Sub-Sub-TLV pass it and then fail `attribute.forEachPrefixSIDTLV` in every reader. The two collapse into the `attribute` declaration (`ai/rules/principles.md`, declared once), and the stray-octet case gets its own test (`TestRFC9252SIDInformationStrayOctetsTreatAsWithdraw`).
  → Constraint: tagged `TestRFC8669MalformedAttributeDiscardedAndNotAdvertised` asserts the exact `Description` "RFC 8669 Section 6: Prefix-SID TLV length exceeds attribute bounds", and tagged `TestRFC8669TrailingBytesDiscarded` asserts it contains "trailing bytes". The classifier keeps both strings byte-identical for the non-SRv6 framing verdicts, so neither unit changes; a different wording needs its own `./le rfc approve unit` row first.
- [ ] `internal/core/bgp/attribute/prefixsid_wire.go` - `ParsePrefixSID`, `validatePrefixSIDTLVs`, `validateSRv6ServiceTLV`, `validateSRv6SIDInformation`, `forEachPrefixSIDTLV`, `PrefixSIDTLVSingleOccurrence`, `prefixSIDLabeledUnicast`, `appendPrefixSIDJSON`
  → Constraint: `ParsePrefixSID` refuses an empty attribute, a Label-Index of length other than 7 and an SRGB of length other than 2 + 6N on EVERY family, and a SID Structure of length other than 6. It has no family. Its error propagates from `wire.go::AttributesWire.parseSpan` to every reader of the parsed attribute set, including `packWithContext`.
  → Decision: the family-dependent length rules move out of `ParsePrefixSID` into the family-aware classifier (Design); `ParsePrefixSID` keeps framing and the SRv6 Service structure, which are family-independent.
- [ ] `internal/component/bgp/reactor/rfc8669_duplicate_tlv.go` - `discardRepeatedPrefixSIDTLVs`, `prefixSIDRouteFamily`
  → Decision: `prefixSIDRouteFamily` (MP_REACH family, else IPv4 unicast) is the one answer to "which family is this Prefix-SID attached to"; it moves to `internal/core/bgp/attribute` as a function of (MP_REACH present, AFI, SAFI), so the RFC 7606 step can pass the family its walk already holds, and the D6 discard and the egress clear read the same declaration.
- [ ] `internal/component/bgp/reactor/session_prefix_sid.go` - `clearTransmittedLabelIndex`, called from `session_write.go::writeUpdateBody` on every outbound body (the private copy in `s.writeBuf`), relay and raw writes included
  → Constraint: it clears Reserved and Flags of every 10-octet type-1 TLV on every family; it reads no family. Off labeled unicast a type 1 is not recognized (owner ruling 2026-10-03, RFC 8669 Section 3.1 "It MUST be ignored when received for other BGP AFI/SAFI combinations."), and RFC 8669 Section 6 says "For future extensibility, unknown TLVs MUST be ignored and propagated unmodified." So today Ze rewrites an unrecognized TLV on IPv4 VPN and every other family. AC-1a's fixture (a 6-octet Label-Index, 9 octets with its header) cannot see this; AC-10 can.
  → Decision: the clear runs only when the body's family, read from the moved family declaration, is IPv4 or IPv6 labeled unicast. RFC 8669 Section 3.1: "The Flags field MUST be clear on transmission and MUST be ignored on reception." binds only where the TLV is recognized.
- [ ] `internal/component/bgp/plugins/rib/pool/srv6sid.go` - `ExtractSRv6SID`, `ExtractSRv6SIDFull`, `extractSIDFromServiceTLV`, `parseSIDStructure`, `ApplyTransposition`
  → Constraint: `parseSIDStructure` answers (found, offset, length); a failed sum check answers "not found", which the caller reads as "no transposition, SID whole". A SID Structure shorter than 6 octets is skipped the same way, and one longer than 6 is accepted.
  → Constraint: `ExtractSRv6SIDFull` has no family and takes the first L3 or L2 Service TLV that yields a valid SID, so a VPN route whose L3 TLV holds no SID uses an L2 TLV's SID. RFC 9252 Section 5: "The SRv6 Service SID is encoded as part of the SRv6 L3 Service TLV." This cross-type, family-blind choice is outside D5 and is recorded in `plan/journal/silent-fall-through.md` (2026-10-14); this spec does not pin it with an RFC tag.
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `entrySRv6SID`, `srv6SIDFromResult`, `labelWidthForSAFI`, `isSRv6Ineligible`, `extractSRv6SIDResultFromOtherAttrs`
  → Constraint: the transposition-width bound lives only in `srv6SIDFromResult`, after selection; `labelWidthForSAFI` answers 24 for EVPN and 20 for every other family, including families with no label field. `nlrisplit.TranspositionLabel` reads a label only for IPv4/IPv6 VPN, so the two disagree on which families have a transposition field.
  → Constraint: `isSRv6Ineligible` answers eligible when `pool.OtherAttrs.Get` returns an error, which fails open.
  → Decision: the rewritten predicate answers ineligible on that error and logs it, because a path whose attributes Ze cannot read cannot be shown to carry a valid SID (`ai/rules/principles.md`, no silent default). Unit `TestIsSRv6IneligibleUnreadableOtherAttrsFailsClosed`.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `candidateAdmitted`, the only best-path admission gate, has `fam`
  → Constraint: the SRv6 exclusion is the first test in `candidateAdmitted`, before the RFC 4271 Section 5.1.3 self-next-hop exclusion; the order stays, and `spec-srv6-bestpath-resolvability` adds its check beside it.
- [ ] `internal/component/bgp/plugins/rib/rib_flowspec_validation.go` - `reconcileFlowSpecs` is the second caller of `isSRv6Ineligible`, with `fam` in scope
  → Constraint: both callers change together when `isSRv6Ineligible` gains the family argument.

**Behavior to change:** D1 to D5 above, and the family-blind egress Label-Index clear (AC-10). D6 is done.

**Behavior to preserve:**

| Behavior | Where proven today |
|----------|--------------------|
| Unknown TLVs propagated unmodified at every level | `TestRFC8669UnknownTLVIsIgnored`, `TestRFC8669SingleOccurrencePrefixSIDKeptWhole` |
| A TLV overrun or trailing octets in a non-SRv6 TLV is attribute-discard | `TestRFC8669MalformedAttributeDiscardedAndNotAdvertised`, `TestRFC8669TrailingBytesDiscarded` |
| RFC 7606 Section 5.2 escalation to session-reset when the UPDATE has no reachable NLRI | the walk's tail in `ValidateUpdateRFC7606AddPath` |
| The D6 discard runs after the RFC 7606 walk, so a malformed repeat is withdrawn first | `TestRFC8669DuplicateMalformedServiceTLVStillWithdrawn` |
| A valid SID with valid transposition is reconstructed from the label | `TestSRv6TranspositionRestoresFunctionBitsFromNLRILabel`, `TestSRv6TranspositionTracksTheLabelValue` |
| Reserved octets and unknown endpoint behaviors do not invalidate a SID | `TestExtractSRv6SID_ServiceReservedIgnored`, `TestExtractSRv6SID_UnknownEndpointBehavior` |
| No allocation on the receive path when nothing is wrong | `TestDiscardRepeatedPrefixSIDTLVsAllocatesNothingWithoutRepeat` (the new step owes the same) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a BGP UPDATE received from a peer, carrying path attribute 40 (Prefix-SID)

### Transformation Path
1. Session read, then `enforceRFC7606` runs `message.ValidateUpdateRFC7606AddPath`. The per-attribute walk checks the flags of attribute 40 and records the range of its first occurrence, as it records the AS_PATH range; it does not read the value. After the walk, one classification of that range runs with the family the walk already holds in `mpReachNLRI` (D1, D2, and Q6), unless the walk already recorded a flags conflict for code 40. Its action (attribute-discard or treat-as-withdraw) is applied by the existing enforcement layer.
2. `publishBase`: `discardRepeatedPrefixSIDTLVs` (D6), then publication of the cached bytes.
3. Relay (`forwardUpdateCore`, `reactorForwardRS`) re-emits the published bytes: a discarded attribute is absent, a withdrawn route goes out as a withdrawal. `writeUpdateBody` then runs `clearTransmittedLabelIndex` on the private copy, on IPv4/IPv6 labeled unicast only (AC-10).
4. RIB plugin insert stores the attribute bytes in OtherAttrs.
5. `candidateAdmitted` → `isSRv6Ineligible(fam, entry)` → `pool.ExtractSRv6SIDFull` (D3, D5) plus the label-width bound (D4) decides admission.
6. `entrySRv6SID` → `srv6SIDFromResult` reconstructs the SID for the winning path and feeds sysrib and the FIB.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session ↔ message validator | path attribute bytes, `RFC7606ValidationResult` back | Yes: read `ValidateUpdateRFC7606AddPath` and its result fields 2026-10-13 |
| `message` ↔ `core/bgp/attribute` | `message` already imports `attribute` (`attribute.ValidNextHopLens`, `attribute.AttrTunnelEncap`) | Yes |
| Engine ↔ RIB plugin | attribute wire bytes in the route bundle (`pool.OtherAttrs`) | Yes: `isSRv6Ineligible` reads `pool.OtherAttrs.Get` |
| RIB ↔ sysrib/FIB | `srv6SIDFromResult` result in the best-change event | Yes: `entrySRv6SID` |

### Integration Points
- `candidateAdmitted` and `reconcileFlowSpecs` call `isSRv6Ineligible`; `entrySRv6SID` → `srv6SIDFromResult` feeds the FIB.
- `discardRepeatedPrefixSIDTLVs`, the new RFC 7606 step and `clearTransmittedLabelIndex` read the route family from one declaration.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the malformed decision stays in the RFC 7606 walk; admission stays in `candidateAdmitted` |
| No unintended coupling | Yes | `message` and `reactor` already import `core/bgp/attribute`; the RIB plugin keeps its own `pool` extractor and imports nothing new |
| No duplicated functionality | Yes, after this spec | today two SRv6 Service validators exist (`message/rfc7606.go`, `attribute/prefixsid_wire.go`) and they disagree (Current Behavior); the Label-Index and SRGB length rules exist only in `attribute/prefixsid_wire.go`, and the receive walk has none. This spec deletes the `message` copy |
| Zero-copy preserved | Yes | the classifier walks the received slice; it returns a verdict, no copy |
| Registration over hardcoding, outbound | N-A | no command, family or handler is added |
| Registration over hardcoding, inbound | Yes | which TLV types are recognized per family stays declared once beside `PrefixSIDTLVSingleOccurrence` (`prefixSIDLabeledUnicast`); searched `validatePrefixSIDAttr`, `validatePrefixSIDTLVs`, `extractSIDFromServiceTLV`, `isSRv6Ineligible` for type literals |

## Design

### Alternatives

| Approach | How it works | Gains | Costs |
|----------|--------------|-------|-------|
| A. Patch each producer in place | add the type-1/type-3 length checks to `validatePrefixSIDAttr`, read the type before the overrun test, flip `parseSIDStructure` to invalidate, add a width check to `isSRv6Ineligible` | smallest diff | a third copy of the TLV length rules, family-blind (so a Label-Index on an SRv6 VPN route would be discarded, contradicting the 2026-10-03 owner decision and RFC 8669 Section 3.1), and `ParsePrefixSID` still refuses what the receive path accepted |
| B. One family-aware classifier in `core/bgp/attribute` (RECOMMENDED) | `attribute` gains one exported verdict over a Prefix-SID value and its route family: well-formed, malformed attribute (RFC 8669 Section 6), malformed SRv6 Service TLV (RFC 9252 Section 7), and invalid (no Label-Index on labeled unicast, Q1 (a)). The RFC 7606 walk records the first code-40 range and calls it once after the walk; `ParsePrefixSID` keeps only the family-independent part; the `message` copy of the SRv6 validator is deleted. On the RIB side one predicate answers "valid SRv6 SID for this family" for both admission and reconstruction | every rule declared once, family read from one function, the reader and the validator cannot disagree | touches two packages on the receive side, and `ParsePrefixSID` stops refusing type 1/3 length errors (it never sees them on labeled unicast, because the receive step discarded them) |
| C. Validate in the RIB only | leave the wire validator; have the RIB drop invalid paths | no receive-path change | does not fix D1 or D2 at all: the relay forwards the malformed attribute and the wrong action is still taken; rejected |

→ Decision: B. It is the only approach that keeps one declaration per fact (`ai/rules/principles.md`) and agrees with the owner's family ruling for D6.

### Proposed behavior

| Defect | Today | After |
|--------|-------|-------|
| D1 | type 1 or 3 of a non-conforming length, and an empty attribute, accepted and relayed | on IPv4/IPv6 labeled unicast: attribute-discard, logged with the peer, not relayed. Off labeled unicast types 1 and 3 are unrecognized and relayed unmodified whatever their length. Empty attribute: attribute-discard on every family (Q2 (a)) |
| D2 | type 5 or 6 overrunning the attribute: attribute-discard | treat-as-withdraw on every family; the type octet is read before the overrun test. Off labeled unicast, other framing errors follow Q6 (a) |
| Egress clear (AC-10) | Label-Index Reserved and Flags cleared on every family | cleared on IPv4/IPv6 labeled unicast only; on every other family a type 1 TLV leaves Ze octet-equal to what was received. Order: the existing TLV walk runs first, and the family is looked up (`prefixSIDRouteFamily` after its move, one `AttrFind(MP_REACH)` over the attribute headers, no value parse, no allocation) only when the walk meets a 10-octet type 1 TLV whose Reserved or Flags are non-zero, once per body. When no family is readable (MP_REACH shorter than 3 octets, the `!ok` answer) the clear does nothing and the body goes out unmodified, the same action the function already takes on a framing it cannot read. Why: under the owner's 2026-10-03 ruling a type 1 TLV is recognized only on labeled unicast, so with no family there is no evidence the TLV is recognized, and RFC 8669 Section 6 says "For future extensibility, unknown TLVs MUST be ignored and propagated unmodified."; clearing would rewrite a TLV Ze cannot show it owns. The case is not reachable from a received body, because the RFC 7606 walk already resets the session on that MP_REACH (Security Review Checklist, "Fail closed"), so it arises only from a local encoder defect |
| D3 | invalid SID Structure: transposition dropped, SID used whole | the SID of that SID Information Sub-TLV is invalid; nothing is installed from it. A SID Structure Sub-Sub-TLV whose length is not 6 also fails Section 3.2.1 and invalidates the SID (it is not malformed: RFC 9252 Section 7 lists only an inconsistent length) |
| D4 | admission asks extraction only, no width bound, no family | admission asks one predicate with the family: SID valid after D3, and transposition length within `labelWidthForSAFI`; a non-zero offset with zero length, or any transposition on a family outside the transposition set named in Q4.1, is invalid too (Q4.1 (a)); a SID Structure with AL != 0 is invalid (Q4.2 (a)) |
| D5 | first Service TLV with a valid SID wins, across instances | only the first instance of type 5 and the first instance of type 6 are consulted; a later instance of the same type is skipped whatever it holds |

### Failure modes → recorded as R-N rows in Risks & Assumptions.

### Triple challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | B adds one classifier and deletes one validator; the RIB change is an argument and a shared predicate. The family-aware check is the minimum that respects RFC 8669 Section 3.1 and the 2026-10-03 ruling |
| Uniformity | same shape as D6: a family-aware Prefix-SID rule in `core/bgp/attribute`, called from the receive path; the post-walk step mirrors the existing mandatory-attribute checks at the end of the walk |
| Performance | the walk records the code-40 range without reading the value, and the classifier then walks the value once, no allocation on the accept path (a test asserts `AllocsPerRun` 0 like D6); `isSRv6Ineligible` already walks OtherAttrs per candidate, the width compare adds no allocation. Egress (AC-10, per outbound body): a body with no dirty 10-octet type 1 TLV pays nothing new; one that has one pays one extra `AttrFind(MP_REACH)` header walk, no value parse, no allocation (`TestClearTransmittedLabelIndexOnlyOnLabeledUnicast` asserts `AllocsPerRun` 0 on both families) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE from a peer, labeled unicast, Label-Index TLV of length 6 (D1) | → | `enforceRFC7606` → `ValidateUpdateRFC7606AddPath` → family-aware Prefix-SID step | `test/plugin/prefixsid-malformed-receive.ci` |
| UPDATE from a peer, IPv4 VPN, SRv6 L3 Service TLV overrunning the attribute (D2) | → | same step, treat-as-withdraw | `test/plugin/prefixsid-malformed-receive.ci` |
| UPDATE from a peer, labeled unicast, Prefix-SID with only an SRGB TLV (AC-1b, Q1 (a)) | → | same step, invalid attribute | `test/plugin/prefixsid-malformed-receive.ci` case (d) |
| UPDATE from a peer carrying an empty attribute 40 (AC-1c) | → | same step, attribute-discard on every family (Q2 (a)) | `test/plugin/prefixsid-malformed-receive.ci` case (e) |
| UPDATE from a peer, IPv4 VPN, 7-octet Label-Index with non-zero Flags, relayed (AC-10) | → | `writeUpdateBody` → `clearTransmittedLabelIndex` family gate | `test/plugin/prefixsid-malformed-receive.ci` case (f) |
| UPDATE from a peer, IPv4 VPN, SID Structure with sum above 128 (D3, D4) | → | `candidateAdmitted` → `isSRv6Ineligible(fam, entry)` → `pool.ExtractSRv6SIDFull` | `test/plugin/srv6-invalid-sid-ineligible.ci` |
| UPDATE from a peer, IPv4 VPN, transposition length 24 against a 20-bit label (D4) | → | `isSRv6Ineligible(fam, entry)` width bound | `test/plugin/srv6-invalid-sid-ineligible.ci` |
| UPDATE from a peer whose Prefix-SID repeats a single-occurrence TLV (D6) | → | `publishBase` → `discardRepeatedPrefixSIDTLVs` | `test/plugin/prefixsid-duplicate-tlv-relay.ci` |

## Acceptance Criteria

Each AC names its proof. "Unit" means a test beside the producer; ".ci" and "interop" name the files in the TDD Test Plan.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPv4 or IPv6 labeled unicast UPDATE whose Prefix-SID holds a Label-Index TLV of length 6 or 8, or an Originator SRGB TLV of length 2, 3 or 9 (8 = 2 + 6 is valid) | result action attribute-discard for code 40, the route itself installed and relayed without attribute 40, a log line naming the peer and RFC 8669 Section 6. Proof: unit `TestRFC8669LabelIndexLengthMalformedOnLabeledUnicast`, `TestRFC8669SRGBLengthMalformedOnLabeledUnicast`; `.ci` `prefixsid-malformed-receive.ci`; interop `bgp-prefix-sid-malformed-frr` |
| AC-1a | the same TLVs (Label-Index of length 6, SRGB of length 3) on an IPv4 VPN or IPv6 unicast route | no error, action none, attribute 40 relayed octet-equal (RFC 8669 Section 3.1 "MUST be ignored"; RFC 9252 Section 7, unrecognized Type is not malformed); `ParsePrefixSID` succeeds on it. Proof: unit `TestRFC8669LabelIndexAnyLengthIgnoredOffLabeledUnicast`, `TestRFC8669SRGBAnyLengthIgnoredOffLabeledUnicast`, `TestParsePrefixSIDAcceptsOffFamilyLabelIndexLength` |
| AC-1b | (Q1 (a)) labeled unicast Prefix-SID with no Label-Index TLV (for example only an SRGB TLV) | attribute-discard as AC-1, the route relayed without attribute 40. Off labeled unicast, the same attribute is kept. Proof: unit `TestRFC8669NoLabelIndexInvalidOnLabeledUnicast` both polarities; `.ci` `prefixsid-malformed-receive.ci` case (d); interop `bgp-prefix-sid-malformed-frr` (AC-1b prefix) |
| AC-1c | empty attribute 40 (length 0) | attribute-discard on every family (Q2 (a)). Proof: unit `TestRFC8669EmptyAttributeMalformed`; `.ci` `prefixsid-malformed-receive.ci` case (e) |
| AC-2 | SRv6 L3 (type 5) or L2 (type 6) Service TLV whose length runs past the end of attribute 40, on IPv4 VPN and on IPv6 unicast; and, inside a SID Information Sub-TLV, 1 or 2 octets after the last Sub-Sub-TLV | action treat-as-withdraw, attribute code 40, description naming RFC 9252 Section 7; the routes of that UPDATE are withdrawn and the relay sends a withdrawal. A type 1 or unknown TLV overrunning the attribute, and trailing octets after the last TLV: on labeled unicast attribute-discard; off labeled unicast per Q6 (a): treat-as-withdraw when a type 5/6 TLV was read before the error or is the TLV that breaks the framing, attribute-discard otherwise. Proof: unit `TestRFC9252ServiceTLVOverrunTreatAsWithdraw` (types 5 and 6, both families), `TestRFC9252SIDInformationStrayOctetsTreatAsWithdraw`, `TestRFC7606PrefixSIDFramingErrorAfterServiceTLV` (Q6), and the existing attribute-discard tests; `.ci` `prefixsid-malformed-receive.ci`; interop `bgp-prefix-sid-malformed-frr` |
| AC-3 | SID Structure Sub-Sub-TLV with LBL+LNL+FL+AL above 128, or below Transposition Offset + Transposition Length, or of a length other than 6, or Transposition Length 0 with Offset non-zero (Q4.1 (a)), or AL != 0 (Q4.2 (a)) | `ExtractSRv6SIDFull` answers no SID (`SID.IsValid()` false). The boundary sum equal to offset+length and sum 128 stay valid (erratum 7817). Proof: unit `TestExtractSRv6SIDFull_InvalidSIDStructure`, `TestExtractSRv6SIDFull_SumBelowTransposition` (both changed, see Q5), `TestExtractSRv6SIDFull_WithTransposition` (unchanged), new `TestExtractSRv6SIDFull_SIDStructureLengthNotSix`, new `TestExtractSRv6SIDFull_ZeroLengthNonZeroOffset` (Q4.1), new `TestExtractSRv6SIDFull_NonZeroArgumentLengthInvalid` (Q4.2) |
| AC-4 | one path for a prefix, whose Prefix-SID carries SRv6 Service TLVs and whose only SID is invalid by AC-3, or whose transposition length exceeds the label width of its family (21 or more on VPN, 25 or more on EVPN), or any non-zero transposition on a family outside the transposition set (Q4.1 (a)) | `candidateAdmitted` returns false: zero real candidates for the prefix, no best-change, nothing installed. A path with a valid SID and transposition length exactly 20 (VPN) stays eligible. Proof: unit `TestIsSRv6Ineligible_InvalidSIDStructure`, `TestSRv6TranspositionWiderThanLabelFieldIsIneligible` (extended to assert admission), `TestSRv6TranspositionAtLabelWidthIsEligible`; `.ci` `srv6-invalid-sid-ineligible.ci` (P1, P2) |
| AC-4a | two paths for one prefix: the better-attributed one invalid as in AC-4, the other with a valid SID | the valid path is the best path and its SID is installed. Proof: interop `bgp-srv6-invalid-sid-ineligible-frr` |
| AC-5 | Prefix-SID value with two L3 Service TLVs, the first carrying no valid SID (SID Information under 21 octets, or none, or an invalid SID Structure) and the second a valid SID; the same for two L2 Service TLVs | `ExtractSRv6SIDFull` answers no SID; the second instance is never read. One L3 then one L2 Service TLV each consulted once. Proof: unit `TestRFC9252SecondL3ServiceTLVIgnoredWhenFirstHasNoSID`, `TestRFC9252SecondL2ServiceTLVIgnoredWhenFirstHasNoSID`. On the wire D6 already removes the second instance at ingest (`prefixsid-duplicate-tlv-relay.ci`), so no `.ci` reaches this reader with a repeat (A-3) |
| AC-6 | the `weak` verdicts of RFC8669-6-1, RFC9252-3.4-1, RFC9252-3.2.1-3, RFC9252-5-1 and RFC9252-7-1, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3, 2026-09-28) |
| AC-7 | the `weak` verdicts of RFC9252-7-2 (the L2 Service half of D5), RFC8669-3.1-2 and RFC8669-3.2-4 (both turn on D1's validator), after this spec's producer fix | each verdict reaches `enforced`, re-judged as in AC-6. Moved here from the BGP child under parent P-3, 2026-09-30 |
| AC-8 | Received Prefix-SID with repeated recognized single-occurrence TLVs, including Label-Index (on IPv4/IPv6 labeled unicast only, owner decision 2026-10-03) and SRv6 Service types 5 and 6 (every family) | Retain the first instance of each recognized type and discard later instances before re-advertisement; preserve unknown TLVs unmodified. A peer-level receive/relay proof checks the outgoing bytes, not only the extracted SID. The red probe `rfc8669_duplicate_tlv_red_test.go` was kept until the fix made it pass, then removed in `a41bf6a186`. |
| AC-9 | RFC8669-6-3 after AC-8 | Both polarities and native discrimination prove discard of duplicates, and an independent judge stamps the row enforced. This is the BGP child's P-3 transfer approved on 2026-10-02, not an implementation claim by the audit pass. |
| AC-10 | a received IPv4 VPN route whose Prefix-SID holds a Label-Index TLV of length 7 with non-zero Reserved and Flags, relayed to another peer; control: the same TLV on IPv4 labeled unicast | IPv4 VPN: attribute 40 leaves Ze octet-equal to what was received (RFC 8669 Section 6: "For future extensibility, unknown TLVs MUST be ignored and propagated unmodified."). Labeled unicast: Reserved and Flags are cleared (RFC 8669 Section 3.1: "The Flags field MUST be clear on transmission and MUST be ignored on reception."). Proof: unit `TestClearTransmittedLabelIndexOnlyOnLabeledUnicast` (both families, code 40 before and after MP_REACH); `.ci` `prefixsid-malformed-receive.ci` case (f) |

AC-8 and AC-9 are met at HEAD (`a41bf6a186`, `465af38f1d`, `0c6ae07e69`, re-judged `enforced` in `96dbfd4da5`). They stay in the table as record; no work is planned for them.

AC-7 is read as: RFC9252-7-2 follows from AC-5; RFC8669-3.1-2 and RFC8669-3.2-4 follow from AC-1a (a tagged unit feeds a Label-Index and an SRGB on a non-labeled-unicast route through the receive validation and through `ParsePrefixSID`, which is what the RA33 audit note says is missing), and from Q3.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every received attribute 40 passes `ValidateUpdateRFC7606AddPath` before any reader | D6 A-1 (confirmed 2026-10-03): `enforceRFC7606` validates, strips, discards, then publishes | a reader sees a malformed attribute and `ParsePrefixSID` no longer refuses the type 1/3 length on its own | grep the callers of `ParsePrefixSID` and `AttributesWire` construction on receive during phase 1 | unvalidated |
| A-2 | `prefixSIDRouteFamily`'s reading (MP_REACH family, else IPv4 unicast) is the right family for D1 | the owner's 2026-10-03 ruling and its D6 use | an UPDATE carrying both IPv4 NLRI and an MP_REACH of another family is judged by the MP_REACH family only | test both orders of code 40 and MP_REACH; record the mixed case as a Known Limitation, as the D6 judge did | unvalidated |
| A-3 | No path into the RIB delivers a repeated Service TLV after D6 (received routes pass `publishBase`; local origination writes one Service TLV) | `discardRepeatedPrefixSIDTLVs` call site; `ParsePrefixSIDSRv6` encoder | D5 has a live entry point and owes a `.ci` | grep the RIB insert paths (`rib_inject.go`, structured events) for an attribute source that bypasses `publishBase` | unvalidated |
| A-4 | FRR, as receiver, accepts a labeled-unicast route with a Prefix-SID Label-Index from Ze, and shows it with and without the attribute | FRR bgpd implements RFC 8669 label index | the D1 interop needs a different observable (FRR's received-attribute JSON) | first run of `bgp-prefix-sid-malformed-frr` | unvalidated |
| A-5 | The raw injector can send two UPDATEs in sequence from one `inject.msg` (announce, then malformed re-announce) | `bgp-srv6-next-hop-identity-frr` uses `inject.msg`; sequencing not yet read | the D2 interop needs two injectors or a timed second message | read the injector's message-file grammar in phase 4 | unvalidated |
| A-6 | `bgp rib inject` plus `rib best` (the `bestpath-reason.ci` pattern) can show two candidates for one IPv4 VPN prefix | `test/plugin/bestpath-reason.ci` does it for IPv4 unicast | the D4 `.ci` uses two wire prefixes (one invalid, one valid) and asserts only presence and absence | phase 3 | unvalidated |
| A-7 | The general and the route-server relay rails relay an IPv4 labeled-unicast (SAFI 4) UPDATE to the `wkPeer` destination that `rfc8669Relay` builds over `EncodingContextForASN4(true)`, so Q5 row 9's new labeled-unicast helper sees one written body per rail | read (design review r3): `rfc8669Relay` never writes through `wkPeer`'s session; the fwdPool callback captures the bodies, and the row-9 test hands the body to `sendRawUpdateBody` on a separate `newAnnouncePeer` session, which reaches `writeUpdateBody` with no family gate; neither forward rail reads negotiated families (the non-test callers of `NegotiatedCapabilities.Has` are `relay_recovery.go`, `reactor_api_batch.go`, `peer_static_wire.go` and `logEORSuppressed`); the helper fails loudly when nothing arrives | none: no `wkPeer` variant is needed | run row 9 red first and confirm it fails on the clear, not on a missing body | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Code 40 precedes MP_REACH in the attribute order, so a check inside the per-attribute validator would read the wrong family | a unit test with code 40 first goes green for the wrong family | the family-aware step runs after the walk; a unit test covers both orders |
| R-2 | Making type 1/3 family-aware loosens `ParsePrefixSID`; a caller that relied on the refusal (JSON, `packWithContext`) now renders a wrong-length Label-Index | `appendPrefixSIDJSON` decodes 7 octets out of a 6-octet value | the formatter renders a non-conforming type 1/3 as an unknown TLV (hex), test both lengths |
| R-3 | D2 turns an attribute-discard into a withdraw: an UPDATE with no NLRI and an overrunning Service TLV now escalates to session-reset under RFC 7606 Section 5.2 | a `.ci` on MP_UNREACH-only updates resets the session | that is what RFC 7606 Section 5.2 requires; a unit test pins it so it is a decision, not a surprise |
| R-4 | D3 makes more SIDs invalid; paths that today win with a SID installed "whole" lose admission, and a prefix with no other path disappears from the FIB | `fib-srv6-kernel.ci` or SRv6 interop scenarios lose a route | the SIDs those tests use are valid; any that fails is checked against Section 3.2.1 before being changed (never edited to pass) |
| R-5 | `isSRv6Ineligible` runs per candidate on every best-path run; a second OtherAttrs walk would double its cost | benchmark or profile of `gatherCandidatesLocked` | one walk, one predicate; no new allocation (AllocsPerRun 0 test on the predicate) |
| R-6 | The verdicts go stale: tagged tests change, and `./le rfc check` or the audit flags them | `./le rfc check` names changed tagged units | each changed unit gets `./le rfc discriminate-record` in the same phase; re-judging is done by an agent that did not write the test |
| R-7 | Another session edits `rfc7606.go` or `rib_bestchange.go` (srv6-bestpath-resolvability touches `candidateAdmitted`) | `git status` shows foreign hunks in those files | sequence with that spec. This spec changes more than `isSRv6Ineligible`'s signature: it changes the validity predicate (D3), adds the width bound (D4), and withdraws the RFC9252-5-1 negative tag from `TestIsSRv6Ineligible_InvalidSID` (Q5 row 4). See R-9 for the premise this breaks in that spec |
| R-8 | `plan/immediate/spec-bgp-update-propagation-rfc-defects.md` folds `clearTransmittedLabelIndex` into its joined egress walk in `writeUpdateBody` (its R-1: "The walk replaces `clearTransmittedLabelIndex`'s `AttrFind`") as a behavior-preserving change, so it keeps the clear family-blind, while AC-10 gates it to labeled unicast. The two specs disagree on what the egress does to a type 1 TLV off labeled unicast | either spec lands with the other open | landing order: that spec lands after this one, or it carries the AC-10 family gate into its joined walk. Recorded here only; that spec is not edited by this one, and its owner reconciles its R-1 and its "Who else touches this path?" row |
| R-9 | `plan/immediate/spec-srv6-bestpath-resolvability.md` (status design) rests on a premise this spec breaks: its "Behavior to preserve" says "`isSRv6Ineligible` and its `RFC9252-5-1` verdict, unchanged, including both tagged tests.", its A-6 says the RFC9252-5-1 tests keep passing unchanged because it does not modify `isSRv6Ineligible`, and its Required Reading row for `rib_bestchange.go` describes today's predicate | that spec's design review or implementation reads the old predicate | landing order: this spec lands first; that spec then re-reads `isSRv6Ineligible(fam, entry)`, its A-6 and that Required Reading row, and builds on the new predicate and the narrowed tags. Recorded here only; that spec is not edited by this one |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes mis-handled on receive: a valid labeled-unicast Prefix-SID discarded, an SRv6 VPN route withdrawn, or a valid SRv6 path excluded from best path (traffic falls back to another path or is dropped). No session effect except R-3, which is RFC-mandated |
| How is it reverted? | One commit per defect group (receive validation; extractor and admission), each revertible alone |
| Who else touches this path? | `plan/immediate/spec-srv6-bestpath-resolvability.md` (adds a resolvability check beside `isSRv6Ineligible` in `candidateAdmitted`), `plan/immediate/spec-srv6-evpn-label-width.md` (EVPN label field and `labelWidthForSAFI`), `plan/immediate/spec-bgp-update-propagation-rfc-defects.md` (the egress walk in `writeUpdateBody`, R-8) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | receives a labeled-unicast route whose Prefix-SID Label-Index is mis-sized | wire → RFC 7606 walk → family-aware step → attribute-discard → relay without code 40 | `prefixsid-malformed-receive.ci`, `bgp-prefix-sid-malformed-frr` |
| 2 | receives an SRv6 VPN route whose Service TLV overruns the attribute | wire → RFC 7606 walk → treat-as-withdraw → relay sends withdrawal | `prefixsid-malformed-receive.ci`, `bgp-prefix-sid-malformed-frr` |
| 3 | receives an SRv6 VPN route with an invalid SID Structure, plus a valid path for the same prefix | wire → RIB insert → `candidateAdmitted` rejects → valid path selected, its SID installed | `srv6-invalid-sid-ineligible.ci`, `bgp-srv6-invalid-sid-ineligible-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC8669LabelIndexLengthMalformedOnLabeledUnicast` (RFC8669-6-1 negative) | new `internal/component/bgp/message/rfc8669_tlv_length_test.go` | D1: Label-Index of length 6 and 8 on IPv4 and IPv6 labeled unicast, code 40 before and after MP_REACH: action attribute-discard, code 40 | red at HEAD (action none) |
| `TestRFC8669SRGBLengthMalformedOnLabeledUnicast` (RFC8669-6-1 negative) | same | D1: SRGB of length 2, 3 and 9 on labeled unicast: attribute-discard (8 = 2 + 6 is the positive row below) | red at HEAD |
| `TestRFC8669ConformingTLVLengthsAcceptedOnLabeledUnicast` (RFC8669-6-1 positive) | same | D1: Label-Index 7 and SRGB 2+6 and 2+12 on labeled unicast: action none, bytes unchanged | green at HEAD, must stay green |
| `TestRFC8669LabelIndexAnyLengthIgnoredOffLabeledUnicast` (RFC8669-3.1-2 positive) | same | D1/AC-1a: Label-Index of length 6 on IPv4 VPN and IPv6 unicast: action none, attribute kept octet-equal | green at HEAD on the receive walk; the paired `ParsePrefixSID` test below is red |
| `TestRFC8669SRGBAnyLengthIgnoredOffLabeledUnicast` (RFC8669-3.2-4 positive) | same | D1/AC-1a: SRGB of length 3 on IPv4 VPN: action none | as above |
| `TestParsePrefixSIDAcceptsOffFamilyLabelIndexLength` | new `internal/core/bgp/attribute/rfc8669_prefixsid_family_test.go` | AC-1a: `ParsePrefixSID` and `AttributesWire.All` succeed on a 6-octet Label-Index; the JSON renders it as an unknown TLV, not as `sr-label-index` (R-2) | red at HEAD (parse error) |
| `TestRFC8669NoLabelIndexInvalidOnLabeledUnicast` (RFC8669-4.1-2 both polarities, Q1 (a)) | `rfc8669_tlv_length_test.go` | AC-1b | red at HEAD |
| `TestRFC8669EmptyAttributeMalformed` (RFC8669-6-1 negative) | same | AC-1c, Q2 (a): empty attribute discarded on labeled unicast and on IPv4 VPN | red at HEAD |
| `TestClassifyPrefixSIDAllocatesNothing` | `rfc8669_prefixsid_family_test.go` | the classifier on a well-formed value: `AllocsPerRun` 0 | green |
| `TestRFC9252ServiceTLVOverrunTreatAsWithdraw` (RFC9252-3.4-1 negative) | new `internal/component/bgp/message/rfc9252_service_tlv_overrun_test.go` | D2: type 5 and type 6 overrunning attribute 40, on IPv4 VPN and IPv6 unicast: action treat-as-withdraw exactly (not a floor), code 40; a type 1 and an unknown type overrunning stay attribute-discard (paired negative control) | red at HEAD (attribute-discard) |
| `TestRFC9252ServiceTLVOverrunWithoutNLRIResetsSession` | same | R-3: an UPDATE whose only content is that attribute escalates per RFC 7606 Section 5.2 | red at HEAD |
| `TestRFC9252SIDInformationStrayOctetsTreatAsWithdraw` (RFC9252-3.4-1 negative) | same | 1 and 2 octets after the last Sub-Sub-TLV inside a SID Information Sub-TLV whose length covers them: treat-as-withdraw, code 40. RFC 9252 Section 7: a SID Information Sub-TLV is malformed when "At least one of the constituent Sub-Sub-TLVs is malformed.", and a Sub-Sub-TLV is malformed when "The Sub-Sub-TLV Length is inconsistent with the length of the enclosing SRv6 service Sub-TLV." | red at HEAD (the `message` loop accepts it, action none) |
| `TestRFC7606PrefixSIDFramingErrorAfterServiceTLV` (RFC9252-3.4-1, Q6 (a)) | same | Q6 (a) on IPv4 VPN: a valid L3 Service TLV then an unknown TLV overrunning the attribute, and a valid L3 Service TLV then 2 trailing octets: treat-as-withdraw; control: the same errors with no type 5/6 TLV stay attribute-discard | red at HEAD under (a); green under (b) |
| `TestExtractSRv6SIDFull_SIDStructureLengthNotSix` (RFC9252-3.2.1-3 negative) | new `internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go` | D3: lengths 5 and 7 invalidate the SID | red at HEAD |
| `TestExtractSRv6SIDFull_ZeroLengthNonZeroOffset` (RFC9252-3.2.1-1 negative, Q4.1 (a)) | same | D3: length 0 and offset 48: no SID; length 0 and offset 0: SID valid | red at HEAD |
| `TestExtractSRv6SIDFull_NonZeroArgumentLengthInvalid` (RFC9252-3.2-5 negative and positive, Q4.2 (a)) | same | D3: a SID Structure with AL 8: no SID; the same structure with AL 0: SID valid | red at HEAD |
| `TestExtractSRv6SIDFull_Sum128IsValid` (RFC9252-3.2.1-3 positive, boundary) | same | D3: sum exactly 128 with transposition inside it is valid | green |
| `TestRFC9252SecondL3ServiceTLVIgnoredWhenFirstHasNoSID` (RFC9252-7-1 negative) | new `internal/component/bgp/plugins/rib/pool/rfc9252_service_tlv_first_instance_test.go` | D5: first L3 instance with a 20-octet SID Information, with none, and with an invalid SID Structure; second valid: no SID | red at HEAD (second SID returned) |
| `TestRFC9252SecondL2ServiceTLVIgnoredWhenFirstHasNoSID` (RFC9252-7-2 negative) | same | D5 for type 6 | red at HEAD |
| `TestRFC9252FirstL3AndFirstL2BothConsulted` (untagged guard: RFC 9252 Section 7's 7-1 sentence is about repeated L3 instances only and says nothing about L2, so no tag) | same | D5: an L3 instance with no SID then an L2 instance with a SID: the L2 SID is used (each type's first instance is consulted). It guards today's cross-type extraction only; whether that choice is right per family is the `plan/journal/silent-fall-through.md` row of 2026-10-14 | green at HEAD |
| `TestIsSRv6Ineligible_InvalidSIDStructure` (RFC9252-5-1 negative) | new `internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go` | D4: a semantically invalid SID Structure (sum 200), framing well-formed: `candidateAdmitted` false | red at HEAD |
| `TestSRv6TranspositionAtLabelWidthIsEligible` (RFC9252-5-1 positive, boundary) | same | D4: VPNv4, transposition length 20: admitted, SID reconstructed | green |
| `TestSRv6TranspositionOnFamilyWithoutLabelIsIneligible` (RFC9252-3.2.1-2, only with Q4.1 (a)) | same | D4: IPv6 unicast and IPv6 labeled unicast, transposition length 16: not admitted; length 0: admitted | red at HEAD |
| `TestIsSRv6IneligibleAllocatesNothing` | same | R-5 | green |
| `TestIsSRv6IneligibleUnreadableOtherAttrsFailsClosed` | same | a bundle whose OtherAttrs handle no longer resolves (`pool.OtherAttrs.Get` returns an error): not admitted, and the error is logged. Consequence of failing closed: the candidate is excluded whatever its attributes held, so a path with no SRv6 content at all (no attribute 40, which today is admitted because `hasSRv6TLV` stays false) is also excluded when its handle is unreadable; the bytes that would tell the two apart are the ones that cannot be read. Log rate: once per process, carrying the handle and the error. `rib_bestchange.go`'s `peersOverflowed` is the precedent for log-first-then-silent, but it is a plain bool per interner under `peersMu`; `isSRv6Ineligible` is a free function with no lock, so the latch is a package-level `atomic.Bool` taken with `CompareAndSwap(false, true)`, never a copy of the plain-bool shape, which would race; once per handle is not chosen because it needs a set keyed by handle on the per-candidate path, which allocates and grows without bound. Without the latch it would log once per candidate per best-path run. The test asserts the exclusion on two calls and one log line | red at HEAD (admitted) |
| `TestClearTransmittedLabelIndexOnlyOnLabeledUnicast` (RFC8669-3.1-3 and RFC8669-3.1-5 positive on IPv4 and IPv6 labeled unicast; RFC8669-3-1 and RFC8669-3.1-2 positive on IPv4 VPN and IPv4 unicast) | new `internal/component/bgp/reactor/rfc8669_label_index_family_test.go` | AC-10: a 7-octet Label-Index with non-zero Reserved and Flags, code 40 before and after MP_REACH: cleared on labeled unicast, octet-equal on the other families; an MP_REACH shorter than 3 octets: octet-equal (no family, no clear); `AllocsPerRun` 0 | red at HEAD (cleared on every family) |
| `TestRFC8669DuplicateMalformedServiceTLVStillWithdrawn` | `internal/component/bgp/reactor/rfc8669_duplicate_tlv_test.go` | D6: a malformed repeat is still treat-as-withdraw | green |
| `TestDiscardRepeatedPrefixSIDTLVsAllocatesNothingWithoutRepeat` | same | D6: no allocation without a repeat | green |
| `TestRFC8669DuplicateServiceTLVDiscardedOnReceive` (RFC8669-6-3 positive) | same | D6: types 5, 6 and 1 repeated on labeled unicast, type 5 on IPv4 unicast, an unknown TLV after the repeat, an attribute after code 40, extended length, route-server client; published and forwarded bytes | red before, green after; red under review mutants M1, M2, M3 |
| `TestRFC8669SingleOccurrencePrefixSIDKeptWhole` (RFC8669-6-3 negative) | same | D6: each type once, unknown twice, SRGB twice, Label-Index twice on IPv4 unicast published octet-equal | green; red under a family-blind Label-Index mutant |
| `TestPrefixSIDTLVSingleOccurrenceByFamily` | `internal/core/bgp/attribute/prefixsid_wire_test.go` | D6: the declaration per type and family | green |

### RFC-tagged tests that change (owner approval needed, Q5)

Each is edited only after `./le rfc approve unit <package>.<TestName> reason "<owner's words>"`. None is weakened: each asserts more after the change.

| # | Test | File | Tag | Why it must change |
|---|------|------|-----|--------------------|
| 1 | `TestExtractSRv6SIDFull_InvalidSIDStructure` | `internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go` | RFC9252-3.2.1-3 negative | asserts the SID stays valid and is used whole; RFC 9252 Section 7 makes it invalid ("The SRv6 SID value in the SRv6 SID Information Sub-TLV is invalid when ... any of the conditions for the fields of the Sub-Sub-TLV, as specified in Section 3.2.1, is not met."). Becomes: no SID. The tag prose "is rejected (no transposition applied)" is rewritten to "the SID is invalid" |
| 2 | `TestExtractSRv6SIDFull_SumBelowTransposition` | same | RFC9252-3.2.1-3 negative | same reason |
| 3 | `TestIsSRv6Ineligible_ValidSID` | `internal/component/bgp/plugins/rib/rfc9252_srv6_ineligible_test.go` | RFC9252-5-1 positive | `isSRv6Ineligible` gains the family argument; the assertion is unchanged |
| 4 | `TestIsSRv6Ineligible_InvalidSID` | same | RFC9252-5-1 negative | family argument; and its 5-octet SID Information is a structural malformation (treat-as-withdraw upstream), not the Section 7 invalid SID the tag claims (confirmed: `buildInvalidSRv6PrefixSID` writes a SID Information Sub-TLV of length 5). The RFC9252-5-1 negative tag is WITHDRAWN from this unit, which keeps its input and stays as an untagged guard; the 5-1 negative is carried by the new semantic test `TestIsSRv6Ineligible_InvalidSIDStructure` |
| 5 | `TestSRv6TranspositionWiderThanLabelFieldIsIneligible` | `internal/component/bgp/plugins/rib/rfc9252_srv6_transposition_test.go` | RFC9252-5-1 negative | its tag claims ineligibility, its body explicitly excludes candidate admission (audit note); it gains the admission assertion. Its fixture stays isolated: LBL 32 + LNL 16 + FL 24 = 72 = offset 48 + length 24, valid under erratum 7817's ">=", so only the label-width bound fires |
| 6 | `TestRFC9252FirstServiceTLVWins` | `internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go` | RFC9252-7-1 and RFC9252-7-2, negative lines only | OVER-CLAIM at HEAD (journal row in `plan/journal/reference-checked-claim-unchecked.md`): the negative claim "the second ... instance's SID is never the one used" is proven only with a valid first instance, the case HEAD already gets right, while the producer falls back to the second instance when the first yields no SID. The body is unchanged; the two negative lines are narrowed to "with a valid first instance, the second instance's SID is not used", and the full negative is carried by the new D5 tests `TestRFC9252SecondL3ServiceTLVIgnoredWhenFirstHasNoSID` and `TestRFC9252SecondL2ServiceTLVIgnoredWhenFirstHasNoSID` |
| 7 | `TestValidateSRv6ServiceTLV_TrailingBytes` | `internal/component/bgp/message/rfc7606_test.go` | RFC9252-3.4-1 negative | asserts `Description` contains "trailing bytes", a string of the `message` copy of `validateSRv6ServiceTLV` that this spec deletes; the `attribute` validators word their errors differently. The action assertion (treat-as-withdraw) is unchanged; the substring follows the surviving validator's text |
| 8 | `TestValidateSRv6ServiceTLV_SIDInfoTooShort` | same | RFC9252-3.4-1 negative | asserts `Description` contains "< 21", from the deleted copy; the surviving text (`attribute/prefixsid_wire.go`) reads, whole, "prefix-sid: SRv6 SID information sub-TLV is %d octets, RFC 9252 Section 3.1 needs %d before its sub-sub-TLVs". Action assertion unchanged |
| 9 | `TestRFC8669LabelIndexTransmissionClearsReservedAndFlags` | `internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go` | RFC8669-3.1-3 and RFC8669-3.1-5, positive and negative | its fixture (`rfc8669RelayBody`, `rfc8669Relay`) is an IPv4 unicast body, so once AC-10 gates the clear to labeled unicast the dirty case is no longer cleared and the unit goes red. Row 9 gets its own labeled-unicast body and relay helper, new in `rfc8669_label_index_transmit_test.go` (an IPv4 labeled-unicast body, MP_REACH AFI 1, SAFI 4, relayed on both rails); the assertions are unchanged. The shared helpers stay IPv4 unicast and are not edited: `rfc8669RelayBody` and `rfc8669Relay` live in `rfc8669_nexthop_change_defect_test.go` and also serve tagged `TestRFC8669UnknownTLVPropagatedAcrossANextHopChange` (RFC8669-3-1), `TestRFC9252ServiceOnlyPrefixSIDDroppedOnANextHopChange` (RFC9252-3.3-2) and `TestRFC9252ReservedFieldsPropagatedUnchanged` (RFC9252-3.3-1), which read NEXT_HOP (`attrs[3]`) and would break on an MP_REACH body. Premise: A-7; the IPv4 unicast case is carried by `TestClearTransmittedLabelIndexOnlyOnLabeledUnicast`. Found in design review r1 application, 2026-10-14 |

Untagged tests whose assertions change with the design (no approval needed, stated so a reviewer sees them): `TestPrefixSIDRefusesTruncatedTLVs` (`internal/core/bgp/attribute/prefixsid_wire_test.go`), cases "label-index length is not seven", "SRGB length is not 2 plus N times 6", "SRGB carries flags and no range" and "SID structure length is not six" move out of `ParsePrefixSID`'s refusals (AC-1a, AC-3); `TestIsSRv6Ineligible_NoSRv6TLVs` and `TestIsSRv6Ineligible_NoOtherAttrs` take the family argument. The RFC9252-3.4-1 tests (`TestValidatePrefixSIDAttr_Valid`, `TestValidateSRv6ServiceTLV_TrailingBytes`, `TestValidateSRv6ServiceTLV_SIDInfoTooShort`) keep their calls: `validatePrefixSIDAttr` keeps its signature and delegates to the classifier. The last two change their `Description` substring (rows 7 and 8); untagged `TestValidateSRv6ServiceTLV_SubSubTLVExceedsBounds` changes its "sub-sub-TLV exceeds" substring the same way and needs no approval. `TestRFC8669MalformedAttributeDiscardedAndNotAdvertised` and `TestRFC8669TrailingBytesDiscarded` do not change (Current Behavior, `rfc7606.go` constraint). `TestRFC9252FirstServiceTLVWins` changes only in its two negative tag lines (narrowed, TDD row 6, owner-approved under Q5); its body is not edited, and the new D5 cases go to a new file.

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q5). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`: two of these live in `rib/pool`, so their package is `pool`. Every name was checked to exist at HEAD `051876c3f4`; rows 7 to 9 were checked on 2026-10-14. Row 6 narrows two tag lines inside `TestRFC9252FirstServiceTLVWins`, which changes the unit's text, so the gate asks for it even though its body is not edited. The new tests named in this plan are new units and need none.

Each row is one `./le rfc approve unit <unit> reason '<reason>'` command, run once per unit. Every reason opens with the owner's words, `Thomas, 2026-10-13: "approve all RFC-tagged test changes"`, followed by `(spec-bgp-prefix-sid-rfc-defects, <change>)` with the change named below.

| # | Unit | Change named in the reason |
|---|------|----------------------------|
| 1 | `pool.TestExtractSRv6SIDFull_InvalidSIDStructure` | flips to no SID, tag prose rewritten |
| 2 | `pool.TestExtractSRv6SIDFull_SumBelowTransposition` | flips to no SID, tag prose rewritten |
| 3 | `rib.TestIsSRv6Ineligible_ValidSID` | gains the family argument |
| 4 | `rib.TestIsSRv6Ineligible_InvalidSID` | family argument, RFC9252-5-1 negative tag withdrawn |
| 5 | `rib.TestSRv6TranspositionWiderThanLabelFieldIsIneligible` | gains the admission assertion |
| 6 | `pool.TestRFC9252FirstServiceTLVWins` | RFC9252-7-1 and 7-2 negative lines narrowed |
| 7 | `message.TestValidateSRv6ServiceTLV_TrailingBytes` | Description substring follows the surviving attribute validator |
| 8 | `message.TestValidateSRv6ServiceTLV_SIDInfoTooShort` | Description substring follows the surviving attribute validator |
| 9 | `reactor.TestRFC8669LabelIndexTransmissionClearsReservedAndFlags` | fixture moves to IPv4 labeled unicast, assertions unchanged |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Label-Index TLV length (labeled unicast) | exactly 7 | 7 | 6 | 8 |
| Originator SRGB TLV length (labeled unicast) | 2 + 6N, N ≥ 1 | 8 and 14 | 2 (N = 0) and 3 | 9 (not 2 + 6N) |
| LBL+LNL+FL+AL | offset+length to 128 | 128, and sum = offset+length | sum = offset+length − 1 | 129 |
| Transposition length, VPN | 0 to 20 | 20 | N/A | 21 |
| Transposition length, EVPN | 0 to 24 | 24 | N/A | 25 |
| SID Structure Sub-Sub-TLV length | exactly 6 | 6 | 5 | 7 |
| SID Information Sub-TLV length | ≥ 21 (malformed below, existing) | 21 | 20 | N/A |
| Octets after the last Sub-Sub-TLV in a SID Information Sub-TLV | exactly 0 | 0 | N/A | 1 and 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `prefixsid-malformed-receive` | `test/plugin/prefixsid-malformed-receive.ci` | peer 1 sends (a) an IPv4 labeled-unicast route with a 6-octet Label-Index TLV, (b) an IPv4 VPN SRv6 route announced well-formed, then re-announced with a type-5 TLV overrunning attribute 40, (c) control: an IPv4 VPN route with a 6-octet Label-Index and a valid L3 Service TLV, (d) (Q1 (a)) an IPv4 labeled-unicast route whose Prefix-SID holds only an Originator SRGB TLV, (e) a route carrying an empty attribute 40 (on labeled unicast, and on IPv4 VPN, Q2 (a)), (f) control for AC-10: an IPv4 VPN route with a 7-octet Label-Index whose Reserved and Flags are non-zero. Peer 2 (relay target, bgp-rs) must receive (a) without attribute 40, a withdrawal of (b), (c) octet-equal, (d) without attribute 40, (e) without attribute 40 on both families, and (f) with attribute 40 octet-equal to what peer 1 sent. RFC8669-6-1, RFC8669-4.1-2 (d), RFC8669-3-1 and RFC8669-3.1-2 (f) and RFC9252-3.4-1 tags | red at HEAD on (a), (b), (d), (e) and (f) |
| `srv6-invalid-sid-ineligible` | `test/plugin/srv6-invalid-sid-ineligible.ci` | with `bgp-rib`, peer sends three IPv4 VPN prefixes: P1 SID Structure sum 200, P2 transposition length 24, P3 valid with transposition length 20. `rib best` (the `bestpath-reason.ci` observer pattern) answers P3 only; P1 and P2 have no best path. RFC9252-5-1 tag | red at HEAD (P1 and P2 selected) |
- D6: `test/plugin/prefixsid-duplicate-tlv-relay.ci` (RFC8669-6-3 positive), IPv4 unicast: an unknown-twice control and a Label-Index, unknown, Label-Index control both relayed byte for byte; L3 Service, L3 Service, unknown then COMMUNITIES in, L3 Service, unknown then COMMUNITIES out. Red with the discard disabled (seq 3 kept the second L3 TLV) and red with the Label-Index made single on every family (seq 2 lost the second Label-Index), green with the fix.

### Interop Tests
Suite: `./le test integration interop` with `INTEROP_SCENARIO=<name>`. Each red phase is forced by reverting the producer and rebuilding the Ze image (`ai/rules/interop-and-goal-validation.md`).

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-prefix-sid-malformed-frr` | `test/interop/scenarios/bgp-prefix-sid-malformed-frr/` | FRR (receiver, judge), raw injector as sender, Ze as bgp-rs relay (shape of `bgp-prefix-sid-duplicate-tlv-frr`) | D1: FRR holds the labeled-unicast prefix with no Prefix-SID; under HEAD FRR receives the 6-octet Label-Index (its own parser then refuses or logs it, A-4). D2: FRR first holds the VPN prefix, then loses it after the malformed re-announce; under HEAD FRR keeps it (attribute discarded only). AC-1b: FRR holds a second labeled-unicast prefix, sent with a Prefix-SID holding only an SRGB TLV, with no Prefix-SID; under HEAD FRR receives the attribute. Control prefix present throughout | new |
| `bgp-srv6-invalid-sid-ineligible-frr` | `test/interop/scenarios/bgp-srv6-invalid-sid-ineligible-frr/` | FRR as the sender of a valid SRv6 L3VPN path for prefix P (FRR's own SID allocation), raw injector as the sender of a path for P with better attributes (higher LOCAL_PREF over iBGP) and an invalid SID Structure; Ze with `bgp-rib` | D3/D4: Ze's best path for P is FRR's path and the installed SID is FRR's SID (read through Ze's `rib best` in the checker); under HEAD the injector's path wins | new |
| `bgp-prefix-sid-duplicate-tlv-frr` | `test/interop/scenarios/bgp-prefix-sid-duplicate-tlv-frr/` | FRR | D6 (done in `0c6ae07e69`) | exists |

## Files to Modify

- `internal/core/bgp/attribute/prefixsid_wire.go` - the family-aware classifier (one exported verdict); `ParsePrefixSID` keeps only family-independent rules; `appendPrefixSIDJSON` renders a non-conforming type 1/3 as unknown (R-2); `prefixSIDRouteFamily` moves here (exported)
- `internal/component/bgp/message/rfc7606.go` - the walk records the first code-40 range (as it records AS_PATH) and one post-walk call classifies it with the family from `mpReachNLRI`, skipped when the walk already recorded a code-40 flags conflict; `validatePrefixSIDAttr` keeps its signature for its tests and delegates to the same classifier; the local `validateSRv6ServiceTLV` is deleted
- `internal/component/bgp/reactor/rfc8669_duplicate_tlv.go` - reads the moved family function
- `internal/component/bgp/plugins/rib/pool/srv6sid.go` - `parseSIDStructure` reports invalid distinctly from absent; `extractSIDFromServiceTLV` returns no SID on invalid; `ExtractSRv6SIDFull` consults only the first instance of each Service TLV type
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - `isSRv6Ineligible(fam, entry)`; one predicate for "valid SRv6 SID for this family" shared with `srv6SIDFromResult`; `labelWidthForSAFI` answers 0 outside the transposition set (Q4.1 a), derived from the same declaration as `nlrisplit.TranspositionLabel`; the `pool.OtherAttrs.Get` error answers ineligible; stale comment on `srv6SIDFromResult` ("is not done here. isSRv6Ineligible owns that question") rewritten
- `internal/component/bgp/plugins/rib/rib_commands.go` - `candidateAdmitted` passes `fam`
- `internal/component/bgp/plugins/rib/rib_flowspec_validation.go` - `reconcileFlowSpecs` passes `fam`
- `internal/component/bgp/reactor/session_prefix_sid.go` - `clearTransmittedLabelIndex` clears only when the body's family, read from the moved family declaration, is IPv4/IPv6 labeled unicast (AC-10); the family is read only once a dirty 10-octet type 1 TLV is met, and an unreadable family leaves the body unmodified (Proposed behavior, Egress clear row)
- the nine tagged tests and five untagged tests named in the TDD Test Plan
- `internal/le/interoplab/bgp/checkers.go`, `internal/le/interoplab/bgp/names.go` - checkers for the two new scenarios
- `rfc/short/rfc8669.md`, `rfc/short/rfc9252.md` - `{gap}` annotations removed from RFC8669-4.1-1/4.1-2 (Q1 a), RFC9252-3.2.1-1/3.2.1-2 (Q4.1 a) and RFC9252-3.2-5 (Q4.2 a); `## Meta` Support rows
- `rfc/discrimination/rfc8669.json`, `rfc/discrimination/rfc9252.json` - written by `./le rfc discriminate-record`, never by hand
- `docs/features/srv6.md` (RFC tables, and the Propagation row and final-session-writers paragraph for AC-10), `docs/architecture/wire/attributes.md` (section 40, including the egress clear), `docs/architecture/fib/fib-depth-4-srv6.md`, `docs/architecture/wire/messages.md` (`// Design:` of `rfc7606.go`, checked for the Prefix-SID action) - each in the phase that changes its behavior

## Files to Create
- `internal/component/bgp/message/rfc8669_tlv_length_test.go`
- `internal/component/bgp/message/rfc9252_service_tlv_overrun_test.go`
- `internal/core/bgp/attribute/rfc8669_prefixsid_family_test.go`
- `internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go`
- `internal/component/bgp/plugins/rib/pool/rfc9252_service_tlv_first_instance_test.go`
- `internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go`
- `internal/component/bgp/reactor/rfc8669_label_index_family_test.go`
- `test/plugin/prefixsid-malformed-receive.ci`
- `test/plugin/srv6-invalid-sid-ineligible.ci`
- `test/interop/scenarios/bgp-prefix-sid-malformed-frr/` (`ze.conf`, `frr.conf`, `inject.msg`, `inject-args`)
- `test/interop/scenarios/bgp-srv6-invalid-sid-ineligible-frr/` (`ze.conf`, `frr.conf`, `inject.msg`, `inject-args`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config or RPC changes |
| YANG validation constraints | N-A | no leaf added |
| YANG custom validators | N-A | no leaf added |
| CLI commands/flags | N-A | none added; `rib best` is only read by tests |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no command added |
| Functional test for new RPC/API | N-A | no RPC; receive-path `.ci` tests listed above |
| Pipe completeness | N-A | no output added |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no file, socket, port, module or binary added |
| Prometheus counters/metrics | No | the RFC 7606 enforcement layer's existing counters cover discard and withdraw; no new metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | attribute 40 already exists; no new family, capability or code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or scope/evidence/level changed? | Yes | `docs/features/srv6.md` RFC tables (rows named in Required Reading) |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | behavior inside bgp-rib changes; no plugin surface |
| 6 | Has a user guide page? | No | grep `docs/guide/` for Prefix-SID during phase 1; none found by name 2026-10-13 |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md` section 40: receive actions per family; JSON rendering of non-conforming type 1/3 |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8669.md`, `rfc/short/rfc9252.md` and the derived `docs/features/rfc-status.md` (never committed, derived) |
| 10 | Test infrastructure changed? | No | two scenarios added in the existing layout |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fib/fib-depth-4-srv6.md` (extraction now reports invalid SIDs) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Changed source files referenced by doc source anchors? | Yes | run `./le spec citation anchors spec plan/immediate/spec-bgp-prefix-sid-rfc-defects.md` in phase 1; known: `docs/features/srv6.md` anchors `rib_bestchange.go` and `pool/srv6sid.go`; `docs/architecture/wire/messages.md` is the `// Design:` of `rfc7606.go`; `docs/architecture/plugin/rib-storage-design.md` is the `// Design:` of `rib_bestchange.go`; `docs/architecture/fib/fib-depth-4-srv6.md` of `pool/srv6sid.go`; `docs/architecture/wire/attributes.md` of `prefixsid_wire.go` and `rfc8669_duplicate_tlv.go`. Advisory (mention only, 17 pages on 2026-10-13): `docs/architecture/route-selection.md` (ineligible paths, checked in phase 3) and `docs/architecture/bgp/structural-forwarding.md` (ineligible becomes withdrawal, checked in phase 2); the rest (`api/commands.md`, `guide/route-injection.md`, `guide/plugins.md`, and others the command lists) mention `rib_commands.go` or `rib_bestchange.go` for unrelated functions and are named unaffected for that reason |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/wire/attributes.md` JSON examples of `sr-label-index`; checked against the formatter in phase 1 |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- move `prefixSIDRouteFamily` into `attribute`, add the post-walk Prefix-SID step calling an empty classifier, thread `fam` into `isSRv6Ineligible`
   - Tests: the nine tagged edits (after their `./le rfc approve unit` rows) and the failing new tests above, all compiling and red for behavior
   - Files: `prefixsid_wire.go`, `rfc7606.go`, `rfc8669_duplicate_tlv.go`, `rib_bestchange.go`, `rib_commands.go`, `rib_flowspec_validation.go`
   - Verify: `./le job run` of the scoped packages; D6 tests stay green
2. **Phase: Receive validation and egress clear (D1, D2, Q6, AC-10)** -- classifier rules, deletion of the `message` SRv6 validator copy, `ParsePrefixSID` and JSON adjustment, the family gate on `clearTransmittedLabelIndex`
   - Tests: `rfc8669_tlv_length_test.go`, `rfc9252_service_tlv_overrun_test.go`, `rfc8669_prefixsid_family_test.go`, `rfc8669_label_index_family_test.go`, Q5 rows 7 to 9, `prefixsid-malformed-receive.ci`
   - Verify: red then green; discrimination records for every new or changed RFC-tagged unit; docs rows flipped in this phase
3. **Phase: Extraction and admission (D3, D4, D5)**
   - Tests: `rfc9252_sid_structure_validity_test.go`, `rfc9252_service_tlv_first_instance_test.go`, `rfc9252_srv6_admission_test.go`, the edited tagged tests, `srv6-invalid-sid-ineligible.ci`
   - Verify: red then green; `fib-srv6-kernel.ci` and the transposition tests stay green (R-4); discrimination records
4. **Phase: Interop** -- `bgp-prefix-sid-malformed-frr`, `bgp-srv6-invalid-sid-ineligible-frr`, each red with the producer reverted and the image rebuilt, green restored
5. **Phase: Verdicts** -- an agent that wrote none of the tests re-judges RFC8669-6-1, RFC8669-3.1-2, RFC8669-3.2-4, RFC9252-3.4-1, RFC9252-3.2.1-3, RFC9252-5-1, RFC9252-7-1, RFC9252-7-2 with `./le rfc audit-stamp ... mode rejudge` (AC-6, AC-7)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation, named by file and symbol |
| Family | every type-1/type-3 decision reads the moved family function; code 40 before and after MP_REACH both tested |
| One declaration | no second copy of the SRv6 Service validator or of the TLV length rules remains (grep `prefixSIDLabelIndexLen`, `subLen < 21`, `21` literals) |
| Action exactness | tests assert the exact RFC 7606 action, never a floor (`ai/rules/testing.md`) |
| Admission vs reconstruction | `isSRv6Ineligible` and `srv6SIDFromResult` call the same validity predicate |
| No allocation | AllocsPerRun 0 on the classifier and the admission predicate |
| RFC comments | each new implementing function quotes its RFC sentence, each caller names the section (rule dated 2026-09-24) |
| Stale comments | `srv6SIDFromResult`'s "is not done here" comment, `isSRv6Ineligible`'s doc comment, `ParsePrefixSID`'s "fails closed" comment |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Classifier in `attribute`, one copy | `gopls symbols internal/core/bgp/attribute/prefixsid_wire.go`; grep shows no `validateSRv6ServiceTLV` in `message` |
| Family-aware receive step | `TestRFC8669LabelIndexLengthMalformedOnLabeledUnicast` green, `TestRFC8669LabelIndexAnyLengthIgnoredOffLabeledUnicast` green |
| Admission takes the family | `gopls` signature of `isSRv6Ineligible` |
| Verdicts enforced | `rfc/audit/rfc8669.json`, `rfc/audit/rfc9252.json` show `enforced` for the eight rows, stamped by an independent judge |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | every length read from the wire is bounded before indexing; the classifier never reads past the attribute slice; a peer-chosen nesting depth is impossible (three fixed levels) |
| Resource exhaustion | one pass per attribute, no allocation on accept |
| Fail closed | an unreadable family (MP_REACH shorter than 3 octets) is already session-reset by the walk; the step treats a missing answer as an error, never as "not labeled unicast" (`ai/rules/principles.md`) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Two validators of the same attribute existed, one on the receive walk and one in the lazy parser, and they disagreed in both directions: the walk accepted a 6-octet Label-Index the parser refused, and the parser refused a 5-octet SID Structure the walk accepted. Either disagreement turns into an error deep in a reader (`AttributesWire.parseSpan`). Routed to this spec's design (one classifier), no journal row: the fix is in scope here.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Family-aware classifier in `core/bgp/attribute`, called after the RFC 7606 walk | patch in place (A), RIB-only (C) | one declaration; family known only after the walk; matches the 2026-10-03 family ruling |
| Types 1 and 3 unrecognized off labeled unicast, any length relayed | refuse wrong lengths on every family | RFC 8669 Section 3.1 "MUST be ignored", RFC 9252 Section 7 unrecognized Type is not malformed |
| SID Structure length other than 6 makes the SID invalid, not the TLV malformed | treat-as-withdraw | RFC 9252 Section 7 lists only an inconsistent length as malformed, and "not considered malformed because of failing any semantic validation" |
| First instance of each Service TLV type, not first valid | first valid across instances (today) | RFC 9252 Section 7 "all but the first instance MUST be ignored" |

## Known Limitations
- An UPDATE carrying RFC 4271 NLRI and an MP_REACH_NLRI of another family is judged by the MP_REACH family (A-2), as in D6.
- EVPN transposition stays with `plan/immediate/spec-srv6-evpn-label-width.md`; SID resolvability (RFC9252-5-2) stays with `plan/immediate/spec-srv6-bestpath-resolvability.md`.
- RFC9252-3.2-6 binds only a known Endpoint Behavior and Ze knows none; RFC9252-3.2.1-6 binds the advertiser of a SID. Neither has a subject in Ze today (Q4.2).
- `ExtractSRv6SIDFull` picks the L3 or L2 Service TLV without the route family; that is a journal row (`plan/journal/silent-fall-through.md`, 2026-10-14), not this spec's scope.

## Design Review Log

| Round | Date | Reviewer | Result | Disposition |
|-------|------|----------|--------|-------------|
| 1 | 2026-10-14 | independent, did not author (HEAD `68826acc76`) | 0 BLOCKER, 10 ISSUE, 9 NIT | applied. I-1: line numbers replaced by file and symbol. I-2: SRGB malformed set is 2, 3 and 9. I-3: AC-10 gates `clearTransmittedLabelIndex` to labeled unicast, with `.ci` case (f), R-8 and the sequencing table; it also turns tagged `TestRFC8669LabelIndexTransmissionClearsReservedAndFlags` red (IPv4 unicast fixture), added as Q5 row 9. I-4: Q6, AC-2 conditional on it. I-5: Q4 split into Q4.1 and Q4.2. I-6: R-7 corrected, R-9. I-7: `TestRFC9252FirstL3AndFirstL2BothConsulted` untagged, journal row in `plan/journal/silent-fall-through.md`. I-8: `.ci` cases (d) and (e), AC-1b interop case. I-9: three current-state claims corrected, `TestRFC9252SIDInformationStrayOctetsTreatAsWithdraw`. I-10: Q5 rows 7 and 8; the two RFC 8669 framing descriptions are kept byte-identical. N-1 one classification after the walk; N-2 D6 rows moved; N-3 stale prose; N-4 AC-4/AC-4a; N-5 Q2 (b) consequence; N-6 approval table; N-7 transposition set in Q4.1; N-8 Get error fails closed; N-9 flags-conflict skip |
| 2 | 2026-10-14 | independent, did not author; narrow: the round 1 changes only | 0 BLOCKER, 1 ISSUE, 6 NIT | applied. I-1: Q5 row 9 gets its own labeled-unicast body and relay helper; the shared `rfc8669RelayBody`/`rfc8669Relay` stay IPv4 unicast for their three tagged users; A-7 (SAFI 4 relayed to the `wkPeer` destination, unvalidated). N-1: egress family looked up only once a dirty 10-octet type 1 is met; unreadable family leaves the body unmodified (owner ruling 2026-10-03, RFC 8669 Section 6); egress cost in the Performance row; AC-10 unit gains the short-MP_REACH and `AllocsPerRun` cases. N-2: stale S3.2 citation of RFC9252-3.2-5/3.2-6 noted in Q4.2, summary not edited. N-3: Q5 says nine and names row 9. N-4: row 8 quotes the surviving text whole. N-5: belongs to the update-propagation spec, not applied here. N-6: fail-closed consequence and the once-per-process log latch stated |
| 3 | 2026-10-14 | independent, did not author; narrow: the round 2 changes only (`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-prefix-sid-design-r3.md`) | 0 BLOCKER, 0 ISSUE, 2 NIT | applied. NIT-A: A-7 validated (row 9 writes through `sendRawUpdateBody` on its own session, so no `wkPeer` variant). NIT-B: the once-per-process log latch is a package-level `atomic.Bool` with `CompareAndSwap`, since `isSRv6Ineligible` holds no lock. Design converged; Q1..Q4.2 and Q6 stay with the owner |
| owner | 2026-10-14 | Thomas | Q1, Q2, Q3, Q4.1, Q4.2, Q6: accept the recommendation, all (a) | applied: the (b) branches removed from Design, Wiring, ACs, the TDD, functional and interop rows, Files and Known Limitations. No design work owed |

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code, using the sentences quoted in Required Reading.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### Goal Validation
| Goal (Task) | Evidence owed |
|-------------|---------------|
| A malformed Prefix-SID on labeled unicast is not propagated | `bgp-prefix-sid-malformed-frr` green against FRR with its red recorded under the reverted producer; `prefixsid-malformed-receive.ci` hex assertions on the relayed UPDATE |
| A malformed SRv6 Service TLV gets treat-as-withdraw | same scenario: FRR loses the VPN prefix; `.ci` asserts the withdrawal bytes |
| A path with no valid SRv6 SID is not a best-path candidate | `bgp-srv6-invalid-sid-ineligible-frr`: FRR's path wins at Ze over a better-attributed invalid path; `srv6-invalid-sid-ineligible.ci` |
| The eight verdicts reach `enforced` | `rfc/audit/` rows stamped by an independent judge, discrimination records present, `./le rfc check` OK |

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec

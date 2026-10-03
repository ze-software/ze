# Spec: bgp-prefix-sid-rfc-defects

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

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
| D6 | RFC 8669 Section 6 (RFC8669-6-3), RFC 9252 Section 7 | "Similarly, if a recognized TLV appears more than once in a BGP Prefix-SID attribute while the specification only allows for a single occurrence, then all the occurrences of the TLV other than the first one SHALL be discarded and the Prefix-SID attribute will continue to be processed." | Received UPDATE normalization and subsequent Prefix-SID forwarding; preserved probe `internal/component/bgp/reactor/rfc8669_duplicate_tlv_red_test.go` | First-wins extraction alone leaves later recognized single-occurrence TLVs in the relayed attribute. Types 5 and 6 are single-occurrence under RFC 9252 Section 7. | weak; owner transferred this defect here under P-3 on 2026-10-02 |

Related specs, not duplicated here: `plan/immediate/spec-srv6-bestpath-resolvability.md`
owns RFC9252-5-2 (reachability of the SID) at the same candidate filter, and
`plan/immediate/spec-srv6-evpn-label-width.md` owns EVPN transposition. D4 is the
semantic-validity half that spec names as separate work.

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
| - | None open: every row follows its RFC sentence and no reading is in doubt |

On 2026-10-02 Thomas assigned D6 and RFC8669-6-3 to this existing spec rather
than expanding the verdict pass. The earlier five-defect scope is extended by
AC-8/AC-9; D1 through D5 remain unchanged.

On 2026-10-03 Thomas authorised fixing D6 without claiming this spec. That fix
is designed and implemented below (AC-8, and AC-9 up to the independent judge).
D1 to D5, AC-1 to AC-7, and AC-9's `./le rfc audit-stamp ... mode rejudge` are
still open, and the spec stays `skeleton` for them.

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

-> Decision: the single-occurrence set is Label-Index (1), SRv6 L3 Service (5)
and SRv6 L2 Service (6), declared once in
`internal/core/bgp/attribute/prefixsid_wire.go::PrefixSIDTLVSingleOccurrence`.
RFC 9252 Section 7: "If multiple instances of the SRv6 L3 Service TLV are
encountered, all but the first instance MUST be ignored." (and the same for L2).
RFC 8669 Section 1 has the Label-Index TLV "advertise the label index for a
given prefix": one index per prefix.
-> Decision: the Originator SRGB TLV (3) is not in the set. RFC 8669 states no
occurrence limit for it, and Section 3.2 makes repetition its encoding: "the
SRGB field MAY appear multiple times. If the SRGB field appears multiple times,
the SRGB consists of multiple ranges that are concatenated."
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
| Header width kept | the extended-length case in `TestRFC8669DuplicateServiceTLVDiscardedOnReceive` |
| Order kept, unknown kept | the Label-Index case with an unknown TLV between, and `TestRFC8669SingleOccurrencePrefixSIDKeptWhole` |
| Malformed repeat still withdrawn | `TestRFC8669DuplicateMalformedServiceTLVStillWithdrawn` |
| Peer to peer | `test/plugin/prefixsid-duplicate-tlv-relay.ci` |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8669.md`, `rfc/short/rfc9252.md`, and `rfc/full/rfc9252.txt` Sections 3.2.1 and 7

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/rfc7606.go` - `validatePrefixSIDAttr`, `validateSRv6ServiceTLV`
- [ ] `internal/component/bgp/plugins/rib/pool/srv6sid.go` - `ExtractSRv6SID`, `ExtractSRv6SIDFull`, `extractSIDFromServiceTLV`, `parseSIDStructure`
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `isSRv6Ineligible`, `srv6SIDFromResult`, `labelWidthForSAFI`
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `gatherCandidatesLocked`, the candidate filter

**Behavior to change:** the five rows above.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a BGP UPDATE received from a peer, carrying path attribute 40 (Prefix-SID)

### Transformation Path
1. RFC 7606 attribute validation in `internal/component/bgp/message/rfc7606.go`
2. RIB insert; SID extraction in `internal/component/bgp/plugins/rib/pool/srv6sid.go`
3. candidate filter and best-path selection in `internal/component/bgp/plugins/rib/`
4. re-advertisement to other peers and FIB install

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ RIB plugin | attribute wire bytes in the route bundle | No |

### Integration Points
- `gatherCandidatesLocked` calls `isSRv6Ineligible`; `srv6SIDFromResult` feeds the FIB

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE from a peer with a malformed Prefix-SID | → | `validatePrefixSIDAttr`, `isSRv6Ineligible` | [to fill in design: a `.ci` test] |
| UPDATE from a peer whose Prefix-SID repeats a single-occurrence TLV (D6) | → | `publishBase` → `discardRepeatedPrefixSIDTLVs` | `test/plugin/prefixsid-duplicate-tlv-relay.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Prefix-SID with a Label-Index TLV of length 6, an Originator SRGB of length 8, or an empty attribute | attribute discarded, not propagated, error logged |
| AC-2 | type 5 or 6 TLV whose length runs past the attribute | treat-as-withdraw |
| AC-3 | SID Structure failing any Section 3.2.1 condition | the SID is invalid and not installed |
| AC-4 | path whose only SRv6 SID information is invalid, or whose transposition length exceeds the label width | not a best-path candidate |
| AC-5 | two L3 Service TLVs, the first without a valid SID | the second is ignored; no SID is used |
| AC-6 | the `weak` verdicts of RFC8669-6-1, RFC9252-3.4-1, RFC9252-3.2.1-3, RFC9252-5-1 and RFC9252-7-1, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3, 2026-09-28) |
| AC-7 | the `weak` verdicts of RFC9252-7-2 (the L2 Service half of D5), RFC8669-3.1-2 and RFC8669-3.2-4 (both turn on D1's validator), after this spec's producer fix | each verdict reaches `enforced`, re-judged as in AC-6. Moved here from the BGP child under parent P-3, 2026-09-30 |
| AC-8 | Received Prefix-SID with repeated recognized single-occurrence TLVs, including Label-Index and SRv6 Service types 5 and 6 | Retain the first instance of each recognized type and discard later instances before re-advertisement; preserve unknown TLVs unmodified. A peer-level receive/relay proof checks the outgoing bytes, not only the extracted SID. Preserve `rfc8669_duplicate_tlv_red_test.go` until this fix makes it pass. |
| AC-9 | RFC8669-6-3 after AC-8 | Both polarities and native discrimination prove discard of duplicates, and an independent judge stamps the row enforced. This is the BGP child's P-3 transfer approved on 2026-10-02, not an implementation claim by the audit pass. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one failing-first test per defect row, both polarities | beside each producer | D1 to D5 | [to fill in design] |
| `TestRFC8669DuplicateServiceTLVDiscardedOnReceive` (RFC8669-6-3 positive) | `internal/component/bgp/reactor/rfc8669_duplicate_tlv_test.go` | D6: types 5, 6 and 1 repeated, extended length, route-server client; published and forwarded bytes | red before, green after |
| `TestRFC8669SingleOccurrencePrefixSIDKeptWhole` (RFC8669-6-3 negative) | same | D6: each type once, unknown twice, SRGB twice published octet-equal | green before and after |
| `TestRFC8669DuplicateMalformedServiceTLVStillWithdrawn` | same | D6: a malformed repeat is still treat-as-withdraw | green |
| `TestDiscardRepeatedPrefixSIDTLVsAllocatesNothingWithoutRepeat` | same | D6: no allocation without a repeat | green |

### Functional Tests
- [to fill in design: `.ci` over the UPDATE path] (D1, D2, D4)
- D6: `test/plugin/prefixsid-duplicate-tlv-relay.ci` (RFC8669-6-3 positive): Label-Index, unknown, Label-Index in; Label-Index, unknown out; an unknown-twice control relayed byte for byte. Red with the `publishBase` call removed (the second Label-Index reached the receiver), green with it.

### Interop Tests
- [to fill in design: scenario against a peer sending SRv6 L3VPN routes]
- D6: `test/interop/scenarios/bgp-prefix-sid-duplicate-tlv-frr`, not yet written. FRR is the receiver: it treats a repeated SRv6 L3 Service TLV as an error, so an injector sending two type-5 TLVs through Ze to FRR discriminates the discard. Needs an SRv6 L3VPN family between Ze and FRR.

## Files to Modify

- the producers named in the defect table, their tests, and `rfc/short/rfc8669.md` / `rfc/short/rfc9252.md` verdicts

## Implementation Steps

1. Write each failing test, see it red against HEAD.
2. Fix the producer; see it green.
3. Record discrimination for every RFC-tagged unit; re-stamp the audit verdicts.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`

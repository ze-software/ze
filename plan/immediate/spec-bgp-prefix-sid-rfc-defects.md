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

The strict RFC re-read of `plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md`
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

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Prefix-SID with a Label-Index TLV of length 6, an Originator SRGB of length 8, or an empty attribute | attribute discarded, not propagated, error logged |
| AC-2 | type 5 or 6 TLV whose length runs past the attribute | treat-as-withdraw |
| AC-3 | SID Structure failing any Section 3.2.1 condition | the SID is invalid and not installed |
| AC-4 | path whose only SRv6 SID information is invalid, or whose transposition length exceeds the label width | not a best-path candidate |
| AC-5 | two L3 Service TLVs, the first without a valid SID | the second is ignored; no SID is used |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one failing-first test per defect row, both polarities | beside each producer | D1 to D5 | [to fill in design] |

### Functional Tests
- [to fill in design: `.ci` over the UPDATE path]

### Interop Tests
- [to fill in design: scenario against a peer sending SRv6 L3VPN routes]

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

# Spec: bgp-sr-policy-rfc-defects

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

Three defects in Ze's SR Policy SAFI (RFC 9830), found by the strict re-read of
`spec-rfc-requirement-quote-hand-backfill`. Ze sends a
Binding SID with an unassigned flag set, silently truncates a configured
Binding SID label wider than 20 bits and sends one from the reserved range, and accepts a received SR Policy NLRI
that lacks a mandatory field. Each was read at its producer in HEAD on
2026-09-27.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 9830 Section 2.4.2 | "The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt." | `internal/component/bgp/plugins/nlri/srpolicy/config.go::buildBindingSIDSubTLV` | Writes Flags 0x10, an unassigned bit (only S 0x80 and I 0x40 are defined) | no row names it |
| D2 | RFC 9830 Section 2.4.2 (Figure 6), with RFC 3032 Section 2.1 for the label width (no row) | RFC 9830: "If the length is 6, then the BSID is encoded in 4 octets using the format below." and "The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15)." Figure 6 gives Label the first 20 bits, and RFC 3032 Section 2.1 fixes the width: "This 20-bit field carries the actual value of the Label." | `internal/component/bgp/plugins/nlri/srpolicy/config.go` `binding-sid mpls` parse (`strconv.ParseUint` to 32 bits) and `buildBindingSIDSubTLV` (`label << 4`, written through `byte()`); the same width shape in the `type-a mpls` parse and `buildSegmentTypeA` (RFC 9830 Section 2.4.4.2.1: "Label:  20 bits of label value.") | A configured label above 1048575 is accepted and its high bits are lost in the byte truncation, so the peer receives a different label with no error. A Binding SID label 0 to 15 is accepted and sent, against the MUST NOT. A Type A segment label wider than 20 bits truncates the same way | no row; progress DEFECT line; no journal row |
| D3 | RFC 9830 Section 4.2.1 (RFC9830-4.2.1-2) | "The SR Policy NLRI MUST include a distinguisher, Color, and Endpoint field that implies that the length of the NLRI MUST be either 12 or 24 octets (depending on the address family of the Endpoint)." | `internal/component/bgp/plugins/nlri/srpolicy/split.go::SplitSRPolicy` | Frames any non-zero byte-aligned length; an NLRI of another length is accepted on receipt | weak; row has no `{gap}` marker (4.2.1-3 shares the producer) |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1 wire bytes of the sub-TLV; D2 a config with Binding SID label 1048576 refused, label 15 refused, and a `type-a mpls 1048576` segment refused; D3 an NLRI of 13 octets |
| Both polarities | 12- and 24-octet NLRI accepted; Binding SID labels 16 and 1048575 accepted and encoded exactly |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Real entry point | `.ci` tests: config load for D2, a peer sending the NLRI for D3, the sent UPDATE for D1 |
| Interop | an SR Policy scenario against GoBGP or FRR (pathd) receiving Ze's policy |

## Owner decisions

| Row | Question |
|-----|----------|
| - | None open: every row follows its RFC sentence and no reading is in doubt |

D2 has no row of its own in `rfc/short/rfc9830.md`; adding one follows the quote rule.

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9830.md`, `rfc/full/rfc9830.txt` Sections 2.4.2 and 4.2.1

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/config.go` - `buildTunnelEncap`, `buildBindingSIDSubTLV`, the `binding-sid` parse
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/split.go` - `SplitSRPolicy`

**Behavior to change:** the rows above.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- operator config or command announcing an SR Policy; an UPDATE received with SAFI 73

### Transformation Path
1. config parse into the route; Tunnel Encapsulation attribute build
2. on receipt, NLRI split and RIB insert

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ srpolicy plugin | registered NLRI split and route builder | No |

### Integration Points
- the NLRI split registry

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `binding-sid mpls` in config | → | `buildBindingSIDSubTLV` | [to fill in design: `.ci`] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | any announced SR Policy with a Binding SID | Flags octet carries only defined bits |
| AC-2 | `binding-sid mpls 1048576` | refused with a reason |
| AC-3 | received SR Policy NLRI of length other than 12 or 24 | handled as malformed per RFC 9830 |
| AC-4 | the `weak` verdicts of RFC9830-4.2.1-2 and RFC9830-2.4.2-6 (the D1 row: its quote is the row text), after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3, 2026-09-28) |
| AC-5 | the `weak` verdict of RFC9830-4.2.1-7 ("The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message and MUST have a Tunnel Type TLV set to SR Policy (code point is 15)."), a §4.2.1 receive-validation rule: `applyTunnelEncap` now checks present type15/duplicate TLVs, but returns before those checks when attribute23 is absent | a received SR Policy update without the attribute, or without a type-15 TLV, is treated as malformed, a tagged test proves it in both polarities, and an agent that did not write that test re-judges the verdict with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (main-thread ruling, 2026-09-28) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one failing-first test per row | `internal/component/bgp/plugins/nlri/srpolicy/` | D1 to D3 | [to fill in design] |

### Functional Tests
- [to fill in design]

## Files to Modify

- the producers above, their tests, `rfc/short/rfc9830.md`

## Implementation Steps

1. Failing tests; 2. fixes; 3. discrimination and verdicts.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`

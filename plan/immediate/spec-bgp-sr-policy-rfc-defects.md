# Spec: bgp-sr-policy-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three defects in Ze's SR Policy SAFI (RFC 9830), found by the strict re-read of
`spec-rfc-requirement-quote-hand-backfill`. Ze sends a
Binding SID with an unassigned flag set, silently truncates a configured
Binding SID label wider than 20 bits and sends one from the reserved range, and accepts a received SR Policy NLRI
that lacks a mandatory field. Each was read at its producer in HEAD on
2026-09-27.

Re-read at HEAD `a8e502fd51` on 2026-10-13: D1 is already fixed (see "Status
at HEAD"); D2 and D3 remain open; D4, the absent Tunnel Encapsulation attribute
that AC-5 was handed, is now a defect row of its own. The design phase below
is complete up to owner approval; the open questions are in "Owner decisions".

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 9830 Section 2.4.2 | "The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt." | `internal/component/bgp/plugins/nlri/srpolicy/config.go::buildBindingSIDSubTLV` | Writes Flags 0x10, an unassigned bit (only S 0x80 and I 0x40 are defined) | no row names it |
| D2 | RFC 9830 Section 2.4.2 (Figure 6), with RFC 3032 Section 2.1 for the label width (no row) | RFC 9830: "If the length is 6, then the BSID is encoded in 4 octets using the format below." and "The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15)." Figure 6 gives Label the first 20 bits, and RFC 3032 Section 2.1 fixes the width: "This 20-bit field carries the actual value of the Label." | `internal/component/bgp/plugins/nlri/srpolicy/config.go` `binding-sid mpls` parse (`strconv.ParseUint` to 32 bits) and `buildBindingSIDSubTLV` (`label << 4`, written through `byte()`); the same width shape in the `type-a mpls` parse and `buildSegmentTypeA` (RFC 9830 Section 2.4.4.2.1: "Label:  20 bits of label value.") | A configured label above 1048575 is accepted and its high bits are lost in the byte truncation, so the peer receives a different label with no error. A Binding SID label 0 to 15 is accepted and sent, against the MUST NOT. A Type A segment label wider than 20 bits truncates the same way | no row; progress DEFECT line; no journal row |
| D3 | RFC 9830 Section 4.2.1 (RFC9830-4.2.1-2) | "The SR Policy NLRI MUST include a distinguisher, Color, and Endpoint field that implies that the length of the NLRI MUST be either 12 or 24 octets (depending on the address family of the Endpoint)." | `internal/component/bgp/plugins/nlri/srpolicy/split.go::SplitSRPolicy` | Frames any non-zero byte-aligned length; an NLRI of another length is accepted on receipt | weak; row has no `{gap}` marker (4.2.1-3 shares the producer) |

### Status at HEAD (re-read 2026-10-13, HEAD `a8e502fd51`)

| ID | Open at HEAD? | Evidence |
|----|---------------|----------|
| D1 | No, fixed in `a936e853c7` ("rfc: land discriminated runtime proofs and current judgments", 2026-10-06) | `buildBindingSIDSubTLV` no longer writes `buf[2] = 0x10`; the zeroed buffer leaves Flags clear and the function carries the Section 2.4.2 quote. `rfc/audit/rfc9830.json` records RFC9830-2.4.2-6 and -2.4.2-5 `enforced` over `rfc9830_transmit_zero_test.go::TestRFC9830FieldsToIgnoreAreZeroOnTransmission`; `test/encode/bgp-srpolicy-1-encode.ci` expects Binding SID bytes `0D 06 00 00 05 DC 00 00` (Flags 0x00). `plan/pre-release/spec-rfc-verdict-fix-bgp.md` lists 2.4.2-6 under "Historical transfers ... enforced". Nothing remains for D1 here |
| D2 | Yes | `parseConfigRoute` still parses `binding-sid mpls` and `type-a mpls` with `strconv.ParseUint(..., 10, 32)` and no range check; `buildBindingSIDSubTLV` and `buildSegmentTypeA` still shift `label << 4` and write three bytes through `byte()`. RFC9830-2.4.2-11 still carries its `{gap}` marker in `rfc/short/rfc9830.md` |
| D3 | Yes | `SplitSRPolicy` still frames any non-zero byte-aligned length; `validateMPNLRISyntax` (`internal/component/bgp/message/rfc7606.go`) returns nil for SAFI 73; `typedNLRIEdit` leaves before splitting because SR Policy registers no `nlritype` recognizer. RFC9830-4.2.1-2 is `weak`, RFC9830-4.2.1-3 is `{gap}` |
| D4 (AC-5) | Yes | `applyTunnelEncap` (`internal/component/bgp/reactor/session_tunnel_encap.go`) returns unchanged when `result.TunnelEncapPresent` is false, before it computes `policy`, so a SAFI 73 MP_REACH with no attribute 23 is accepted. RFC9830-4.2.1-7 is `weak` |

`./le rfc check` at HEAD (2026-10-13) exits 0 with no RFC 9830 violation. It
lists four orphan discrimination records for RFC9830-2.3-1 and -2.3-3 against
`internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`,
whose tags moved; those are not this spec's verdicts and are not touched here.

D4 is the AC-5 obligation given its own row so it carries a producer:

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong |
|----|-------------|----------------|----------|--------------------|
| D4 | RFC 9830 Section 4.2.1 (RFC9830-4.2.1-7), handled per Section 5 | "The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message and MUST have a Tunnel Type TLV set to SR Policy (code point is 15)." and Section 5: "An SR Policy update that is determined not to be valid (and, therefore, malformed) based on the rules described in Section 4.2.1 MUST be handled by the "treat-as-withdraw" strategy." | `internal/component/bgp/reactor/session_tunnel_encap.go::applyTunnelEncap` | A received SAFI 73 UPDATE with reachable NLRI and no attribute 23 is accepted and installed |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1 already landed with its red (see Status at HEAD). D2 a config with Binding SID label 1048576 refused, label 15 refused, and a `type-a mpls 1048576` segment refused; D3 an NLRI of 13 octets; D4 a SAFI 73 UPDATE without attribute 23 |
| Both polarities | 12- and 24-octet NLRI accepted; Binding SID labels 16 and 1048575 accepted and encoded exactly; a SAFI 73 UPDATE with one type-15 TLV accepted |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Real entry point | `.ci` tests: config load for D2, a peer sending the NLRI for D3 and D4 |
| Interop | an SR Policy scenario against GoBGP receiving Ze's policy (see Interop Tests) |

## Owner decisions

Every question below is open. None is answered, and no answer is assumed by
the design: where a recommendation is given, the AC that depends on it names
the question.

| Row | Question | Options | Recommendation |
|-----|----------|---------|----------------|
| Q1 | D3 scope: RFC 9830 Section 5 also requires session reset for NLRI framing Ze cannot walk (zero length, non-byte-aligned length, an NLRI overrunning the attribute): "Alternately, the router MUST perform "session reset" when the session is only being used for SR Policy or when a "AFI/SAFI disable" action is not possible." Ze has no AFI/SAFI disable (`internal/component/bgp/message/rfc7606_bgpls_nlri.go` says so for BGP-LS). Today such framing is accepted at the session and silently dropped by the RIB. These are the RFC9830-5-1 and -5-4 gap rows, not rows this spec was handed | (a) include: the same registered syntax rule answers both verdicts (AC-6); (b) leave as recorded gaps | (a): one rule, one producer, and leaving it out keeps a silent drop on the same path this spec edits |
| Q2 | Treat-as-withdraw synthesis (`message.SynthesizeWithdrawFamilies`) turns MP_REACH into MP_UNREACH carrying the SAME NLRI bytes and relays it to every eligible peer. For D3 that relays a 13-octet NLRI Ze has just judged malformed | (a) the synthesized SR Policy withdrawal carries only the well-formed NLRIs of the UPDATE, and the malformed ones are dropped from it (nothing in any Adj-RIB-In can be keyed by them once AC-3 holds); (b) relay unchanged, as every other family does | (a), but it adds a family-specific filter to synthesis; if the owner prefers (b), R-2 stands as an accepted risk |
| Q3 | RFC9830-4.2.1-4 and -4.2.1-5 (the update MUST carry NO_ADVERTISE or an IPv4-address Route Target, else malformed) and -4.2.1-8 sit in the same Section 4.2.1 list and would use the same receive hook. They are recorded `{gap}` rows and not in this spec | (a) keep out, they stay gaps; (b) add as AC-7 | (a): scope is the owner's, and (b) changes what every SR Policy peer must send |
| Q4 | D2 width: RFC 9830 states the 20-bit width only as a field description ("Label:  20 bits of label value.", Section 2.4.4.2.1; Figure 6 for the BSID; RFC 3032 Section 2.1 "This 20-bit field carries the actual value of the Label."). There is no MUST, so no requirement row can carry it, and RFC scope is frozen against new enrolment | (a) no new row: the width refusal is proven by untagged regression tests, since a silently truncated value is a `principles.md` defect independent of any row; (b) add a row to `rfc/short/rfc9830.md` | (a) |
| Q5 | D2 reserved range on Type A segments: RFC 9830 forbids labels 0-15 only for the Binding SID (Section 2.4.2). Section 2.4.4.2.1 places no such rule on a Type A segment | (a) Type A keeps accepting 0-15, only the width is checked; (b) refuse 0-15 there too | (a): refusing it would be a rule the RFC does not state |
| Q6 | The new per-family NLRI syntax registry (Design) sits beside the BGP-LS rule that `validateMPNLRISyntax` reaches through the hard-coded `NLRISyntaxRuled` | (a) leave BGP-LS where it is and record the hard-coded branch as a journal row; (b) move BGP-LS onto the registry in this spec, which changes RFC-tagged tests in `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | (a): (b) widens this spec into BGP-LS and its tagged tests |
| Q7 | RFC-tagged test changes listed in the TDD Test Plan, "RFC-tagged tests that change": `TestRFC9830TunnelTypeIsSRPolicy` loses its RFC9830-4.2.1-7 (both) and RFC9830-4.2.1-6 (positive) tags; `TestRFC9830NLRICarriesAllThreeFields` loses its RFC9830-4.2.1-2 tags (both), or has them narrowed if the owner prefers. Both rest on transmit/encoder evidence for Section 4.2.1, which opens "When a BGP speaker receives an SR Policy NLRI from a neighbor, it MUST first perform validation based on the following rules ..."; the new receive tests carry the rows (revised 2026-10-13 after the RFC-tagged test review) | approve or refuse each row | approve |

D2 has no row of its own in `rfc/short/rfc9830.md` (Q4).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/attributes.md` (SR Policy carrier paragraph, lines 132-137)
  → Constraint: the page says actual SR Policy carriers "require exactly one type-15 TLV"; it is silent on an absent attribute. AC-5 makes the page wrong unless the same change states that a SAFI 73 MP_REACH with NLRI and no attribute 23 is treat-as-withdraw
- [ ] `docs/architecture/wire/nlri.md` (family table, SR Policy entry; `split.go` declares it as its `// Design:` page)
  → Constraint: the page names `SplitSRPolicy` as the framing walk; it must gain the receive length rule (96 bits for AFI 1, 192 for AFI 2) and its two verdicts in the same change
- [ ] `docs/features/bgp-protocol.md` lines 36-38 (Binding SID flags note)
  → Constraint: this is where the operator-visible label range belongs: Binding SID 16 to 1048575, Type A 0 to 1048575
- [ ] `docs/architecture/testing/interop.md`
  → Decision: BGP scenarios live in `test/interop/scenarios/<name>/` with a checker registered in `internal/le/interoplab/bgp/checkers.go` / `check_extras.go`; GoBGP checks read `gobgp global rib -a <family> -j` (pattern: `bgp-evpn-gobgp`). No SR Policy scenario exists at HEAD

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9830.md`, `rfc/full/rfc9830.txt` Sections 2.1, 2.4.2, 2.4.4.2.1, 4.2.1 and 5
  → Constraint: Section 5 fixes the action per error class. Well-framed but invalid: "When the error determined allows for the router to skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message, then it MUST handle such malformed NLRIs as 'treat-as- withdraw'." Unwalkable: "Alternately, the router MUST perform "session reset" when the session is only being used for SR Policy or when a "AFI/SAFI disable" action is not possible." Ze has no AFI/SAFI disable, so the second always means session reset
  → Constraint: Section 2.1 ties the length to the AFI, not to the endpoint: "When AFI = 1, the value MUST be 96; when AFI = 2, the value MUST be 192." So a 24-octet body under AFI 1 is malformed even though 4.2.1 lists 24 as a legal length
  → Constraint: Section 5 forbids receive-side field checks: "A BGP implementation MUST NOT perform semantic verification of such fields nor consider the SR Policy update to be invalid or not usable based on such validation." The D2 label checks run on Ze's own origination only, never on receipt
  → Constraint: Section 2.4.2: "The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15)." binds the originator; Section 2.4.4.2.1 says only "Label:  20 bits of label value." for Type A
- [ ] `rfc/full/rfc3032.txt` Section 2.1: "This 20-bit field carries the actual value of the Label."
  → Constraint: any configured label above 1048575 has no encoding and MUST be refused, not truncated
- [ ] `rfc/full/rfc7606.txt` Section 2: treat-as-withdraw means the UPDATE "MUST be treated as though all contained routes had been withdrawn"
  → Decision: an invalid SR Policy NLRI withdraws every route of its UPDATE, not only the bad NLRI

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/config.go` (502L) - `parseConfigRoute` parses every route token; `binding-sid mpls` (around line 137) and `type-a mpls` (around line 281) use `strconv.ParseUint(..., 10, 32)` with no range check; `buildBindingSIDSubTLV` and `buildSegmentTypeA` write `label << 4` into bytes 4..6 through `byte()`, so bits above 20 are lost. Flags already zero (D1 fixed)
  → Constraint: `EncodeRoute` in `encode.go` reuses `parseConfigRoute` (comment at line 43), so config `nlri { ipv4/sr-policy add ... }`, the API `send bgp ... update text ... nlri ipv4/sr-policy` and the ExaBGP bridge (`internal/exabgp/bridge/bridge_command.go` builds the same text) all meet one parser. One check covers every origination entry point
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/split.go` (51L) - `SplitSRPolicy` refuses zero length, non-byte-aligned length and overrun; accepts any other length; ignores ADD-PATH
  → Constraint: the splitter is shared by the RIB walk, diagnostics (`mpDiagnosticNLRI`) and forwarding; making it stricter changes every consumer and the RIB discards its error (`internal/component/bgp/plugins/rib/rib_structured.go`), so strictness in the splitter alone is a silent drop, not an RFC 7606 action
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/types.go` (144L) - `Parse` refuses a body shorter than 12/24 but reads a longer one, ignoring the tail
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/register.go` (45L) - `init()` registers both families' splitter with `nlrisplit.Register` and the plugin registration; no `nlritype` recognizer
  → Decision: the new syntax rule is registered here, beside the splitter, so no core file learns SR Policy
- [ ] `internal/component/bgp/message/rfc7606.go` - `ValidateUpdateRFC7606` calls `validateMPNLRIField` for MP_REACH and MP_UNREACH (line ~447); a session-reset result returns at once, any other result is recorded through `recordError` and becomes the UPDATE's verdict. `validateMPNLRISyntax` (line ~958) handles BGP-LS through the hard-coded `NLRISyntaxRuled`, IPv4/IPv6 unicast and multicast through the prefix walk, and returns nil for every other family, SAFI 73 included
  → Constraint: a treat-as-withdraw verdict from this function already reaches the session's synthesis path; the new rule needs no new reactor plumbing for D3
- [ ] `internal/component/bgp/reactor/session_validation.go` - `rfc7606NLRISyntaxAction` turns a session-reset syntax result into NOTIFICATION 3/10 (Invalid Network Field) per RFC 4271 Section 6.3; `applyTunnelEncap` runs after duplicate stripping (line ~118)
- [ ] `internal/component/bgp/reactor/session_validation_nlritype.go` - `typedNLRIEdit` leaves before splitting any family with no recognizer and no BGP-LS ruling, which is why SAFI 73 framing errors reach no verdict today
- [ ] `internal/component/bgp/reactor/session_tunnel_encap.go` (263L) - `applyTunnelEncap` returns at once when `TunnelEncapPresent` is false; when present it sets `policy` for AFI 1/2 SAFI 73 and refuses a non-15 tunnel type or a second type-15 TLV with `tunnelReceiveError` (treat-as-withdraw, escalated to session reset when the UPDATE has no reachable NLRI)
  → Decision: the AC-5 absent-attribute check goes in this function, at the existing SAFI 73 carrier test, so the type-15 rule and the presence rule share one site
- [ ] `internal/component/bgp/message/rfc7606_withdraw.go` - `SynthesizeWithdrawFamilies` copies the MP_REACH NLRI bytes into MP_UNREACH unchanged (Q2)
- [ ] `internal/component/bgp/reactor/session_prefix_receive_test.go::TestReceivePrefixLimitShortAddPathEntry` - sends SAFI 73 NLRI bytes `08 00` with no attribute 23 and asserts the payload reaches the consumer unchanged
  → Constraint: after AC-3 and AC-5 that UPDATE is treat-as-withdraw, so the fixture must move to bytes that still reach the inventory (R-3)

**Behavior to change:** D2, D3, D4 above (D1 already fixed).

**Behavior to preserve:**
| Behavior | Where proven today |
|----------|--------------------|
| Binding SID Flags, RESERVED, TC/S/TTL zero on transmit | `rfc9830_transmit_zero_test.go`, `rfc9830_test.go::TestRFC9830BindingSIDSubTLV`, `test/encode/bgp-srpolicy-1-encode.ci` |
| one type-15 TLV accepted, other types and two type-15 TLVs treat-as-withdraw | `reactor/rfc9830_tunnel_receive_test.go::TestRFC9830TunnelTypeReceiveVerdicts` |
| an SR Policy withdrawal (MP_UNREACH only) needs no attribute 23 | `reactor/rfc9830_tunnel_receive_test.go::TestTunnelAnnouncementUnaffectedByPolicyWithdrawal` |
| no receive-side semantic check of sub-TLV fields (RFC9830-5-9) | `internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830NoSemanticVerification` |
| labels 16 to 1048575 and Type A 16001, 1048575 accepted and encoded exactly | `rfc9830_test.go`, `encode_test.go`, `config_test.go` fixtures |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Origination: config `nlri { ipv4/sr-policy add ... }`, API `send bgp <sel> update text ... nlri ipv4/sr-policy ...`, ExaBGP bridge `announce ipv4 sr-policy ...`; all reach `parseConfigRoute`
- Receipt: an UPDATE from a peer carrying MP_REACH_NLRI or MP_UNREACH_NLRI with AFI 1 or 2 and SAFI 73

### Transformation Path
1. Origination: route tokens, then `parseConfigRoute` (D2 range checks land here, before any byte is built), then `buildTunnelEncap` and the sub-TLV builders, then the UPDATE builder
2. Receipt: `Session.classifyRFC7606`, then `ValidateUpdateRFC7606`, then `validateMPNLRIField` and `validateMPNLRISyntax`, which consults the per-family syntax registry (D3: the srpolicy rule answers well-formed, malformed-skippable or unwalkable)
3. Receipt, session side: a session-reset verdict leaves through `rfc7606NLRISyntaxAction` as NOTIFICATION 3/10; any other verdict continues; duplicate stripping; `applyTunnelEncap` (D4: SAFI 73 MP_REACH with NLRI and no attribute 23 becomes treat-as-withdraw); typed-NLRI discard; treat-as-withdraw synthesis; RIB

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| srpolicy plugin to the BGP receive validator | the plugin registers its NLRI syntax rule per family in `init()`; `message` reads the registry by family, never by name | No (wiring tests below) |
| config/API to srpolicy plugin | `InProcessConfigRouteParser` / `InProcessRouteEncoder` registrations, unchanged | Yes (existing `.ci` encode tests) |

### Integration Points
- `nlrisplit` registry (unchanged) and the new per-family NLRI syntax registry (core, beside `nlrisplit`)
- `applyTunnelEncap`'s existing SAFI 73 carrier test

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | D3 enters through `validateMPNLRISyntax`, the function that already owns MP NLRI syntax verdicts; D4 through `applyTunnelEncap`, which already owns SAFI 73 tunnel verdicts |
| No unintended coupling | Yes | `message` learns no SR Policy layout; the rule lives in the srpolicy plugin |
| No duplicated functionality | Yes, with Q6 open | the registry is new; BGP-LS keeps its hard-coded route unless the owner picks Q6 (b) |
| Zero-copy preserved | Yes | the rule walks the received slice; it allocates nothing on a well-formed UPDATE |
| Registration over hardcoding, outbound | Yes | the srpolicy plugin registers its rule in `register.go` |
| Registration over hardcoding, inbound | Partly | searched `validateMPNLRISyntax`, `NLRISyntaxRuled`, `typedNLRIEdit`, `nlritype`: SAFI 73 is named in none of them after this change. `applyTunnelEncap` already names SAFI 73 by literal (pre-existing, `policy` flag); D4 extends that site rather than adding a second (Q6 covers the BGP-LS literal) |

## Design

### Alternatives for D3 (receive NLRI length)

| Approach | How | Gains | Costs |
|----------|-----|-------|-------|
| A (recommended) | A per-family NLRI syntax registry in core; `validateMPNLRISyntax` asks it for any family it has no rule for; srpolicy registers a rule per AFI that walks the section and returns well-formed, skippable-malformed (framed, wrong length for the AFI) or unwalkable (zero, non-aligned, overrun) | The verdict reaches the existing RFC 7606 action path; no core file names SR Policy; Q1 comes for free | One new small registry; BGP-LS stays on its own route until Q6 |
| B | Add a SAFI 73 case to `validateMPNLRISyntax` in `message` | Smallest diff | A central enumeration (`principles.md`), and `message` would carry the SR Policy layout |
| C | Register an `nlritype` recognizer for SAFI 73 that returns false on a wrong length | Reuses Section 5.4 machinery; framing errors already reach session reset there | Its action is NLRI discard, not the treat-as-withdraw RFC 9830 Section 5 requires, and it misnames a length rule as an unrecognized type |
| D | Make `SplitSRPolicy` refuse a wrong length | One-line change | The RIB discards the error, so the route vanishes silently; no RFC 7606 action, no log (`principles.md`: silently wrong) |

### Alternatives for D2 (label range)

| Approach | How | Gains | Costs |
|----------|-----|-------|-------|
| A (recommended) | Range checks in `parseConfigRoute` at the two label tokens: BSID 16..1048575, Type A 0..1048575, refused with a message naming the token, the value and the range | Every origination entry point shares the one parser; nothing reaches the builders out of range | none found |
| B | Mask in the builders | No refusal path | Still sends a different label than configured, which is the defect |

### Proposed changes

| Area | Current | Proposed |
|------|---------|----------|
| `binding-sid mpls <n>` | any 32-bit value; high bits truncated; 0..15 sent | 16..1048575 accepted unchanged; 0..15 refused citing the reserved range; above 1048575 refused citing the 20-bit width |
| `type-a mpls <n>` | any 32-bit value, truncated | 0..1048575 accepted; above refused (Q5: 0..15 stays accepted) |
| received SAFI 73 NLRI, framed, length not 96 bits (AFI 1) / 192 bits (AFI 2) | carried to RIB and peers | the whole UPDATE is treat-as-withdraw, logged with the RFC 9830 Section 5 reason; the session stays up |
| received SAFI 73 NLRI, unwalkable (zero, non-aligned, overrun) | carried, then dropped silently by the RIB | session reset with NOTIFICATION 3/10 (Q1) |
| received SAFI 73 MP_REACH with NLRI and no attribute 23 | accepted | treat-as-withdraw, reason names RFC 9830 Section 4.2.1 |
| received SAFI 73 MP_REACH with no NLRI and no attribute 23 | accepted | unchanged: it carries no SR Policy NLRI, so it is not an SR Policy update |
| received SAFI 73 MP_UNREACH only | accepted | unchanged for attribute 23; the length rule applies to its NLRIs too |

### Failure modes considered
Recorded as R-1 to R-6 below.

### Triple challenge
| Challenge | Answer |
|-----------|--------|
| Simplicity | D2 is two range checks at one parser. D3 needs one registry because the alternative is a central case (B) or a wrong action (C, D). D4 is one early-return branch at an existing site |
| Uniformity | The registry mirrors `nlrisplit` and `nlritype`: per-family registration in the plugin's `init()`, read by family in core |
| Performance | The rule runs once per MP attribute of SAFI 73 only; it walks the existing slice with an offset and returns a value; zero allocation on the well-formed path. The D4 branch is a flag test on an already-computed result |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config `ipv4/sr-policy add ... binding-sid mpls 15` | → | `parseConfigRoute` range check | `test/parse/srpolicy-binding-sid-label-range.ci` (config load refused, stderr names the token and range) |
| API `send bgp * update text ... binding-sid mpls 1048575 ... type-a mpls 1048575` | → | `parseConfigRoute`, `buildBindingSIDSubTLV`, `buildSegmentTypeA` | `test/encode/bgp-srpolicy-3-encode.ci` (exact hex of label 0xFFFFF in both sub-TLVs) |
| peer sends SAFI 73 UPDATE with a 13-octet NLRI | → | registered srpolicy syntax rule via `validateMPNLRISyntax` | `test/plugin/srpolicy-receive-nlri-length.ci` and `internal/component/bgp/reactor/rfc9830_nlri_receive_test.go::TestRFC9830NLRILengthReceiveVerdicts` |
| peer sends SAFI 73 UPDATE without attribute 23 | → | `applyTunnelEncap` absent-attribute branch | `test/plugin/srpolicy-receive-no-tunnel-encap.ci` and `internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go::TestRFC9830SRPolicyWithoutTunnelEncapIsMalformed` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | any announced SR Policy with a Binding SID | Flags octet carries only defined bits. Already met at HEAD by `a936e853c7` (RFC9830-2.4.2-6 `enforced`); kept so the preservation tests stay named, no new work |
| AC-2 | `binding-sid mpls 1048576`, `binding-sid mpls 15`, `binding-sid mpls 0`, `type-a mpls 1048576`, through config and through the API | each refused before any UPDATE is built; the message names the token, the value and the allowed range; `binding-sid mpls 16`, `binding-sid mpls 1048575`, `type-a mpls 0`, `type-a mpls 1048575` accepted and encoded with exactly that 20-bit label in bytes 4..6 and TC/S/TTL zero |
| AC-3 | received SAFI 73 NLRI whose length octet is byte-aligned, fits the attribute, and is not 96 for AFI 1 or 192 for AFI 2 (cases: AFI 1 with 13 octets, AFI 1 with 24 octets, AFI 2 with 12 octets, AFI 2 with 25 octets), in MP_REACH and in MP_UNREACH | the UPDATE is treat-as-withdraw: every route it carries is withdrawn from the Adj-RIB-In, the session stays Established, no NOTIFICATION is sent, and the log line names RFC 9830 Section 5 and the offending length. 12-octet AFI 1 and 24-octet AFI 2 NLRIs, alone and beside each other's families in separate UPDATEs, are accepted with no RFC 7606 action. The synthesized withdrawal content follows Q2 |
| AC-4 | the `weak` verdict of RFC9830-4.2.1-2 (RFC9830-2.4.2-6 is already `enforced`, see Status at HEAD) | a tagged receive-side test proves the quoted sentence in both polarities, `./le rfc discriminate-record` records its break, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge` to `enforced`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3, 2026-09-28) |
| AC-5 | the `weak` verdict of RFC9830-4.2.1-7 ("The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message and MUST have a Tunnel Type TLV set to SR Policy (code point is 15)."), a Section 4.2.1 receive-validation rule (D4) | a received SAFI 73 UPDATE whose MP_REACH carries NLRI and which has no attribute 23 is treat-as-withdraw, logged with the RFC 9830 Section 4.2.1 reason, session stays up; with exactly one type-15 TLV it is accepted with no action; a non-15 type or two type-15 TLVs stay treat-as-withdraw as today. A tagged test proves it in both polarities, `./le rfc discriminate-record` records its break, and an agent that did not write that test re-judges the verdict with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (main-thread ruling, 2026-09-28) |
| AC-6 | received SAFI 73 NLRI section that cannot be walked: length octet 0, length not a multiple of 8, or an NLRI overrunning the attribute (only if Q1 = a) | session reset with NOTIFICATION Update Message Error / Invalid Network Field (3/10), logged with the RFC 9830 Section 5 reason; RFC9830-5-1 and -5-4 lose their `{gap}` markers and carry tagged tests |
| AC-7 | `rfc/short/rfc9830.md` after the fix | RFC9830-2.4.2-11 and -4.2.1-3 lose their `{gap}` markers and carry tagged tests in both polarities; the `Support remaining` sentence no longer lists "Reserved MPLS binding-label validation" and no longer says complete SAFI 73 NLRI validation is not established for the length rule; `docs/features/rfc-status.md` regenerated from it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9830BindingSIDLabelRange` (new; tags RFC9830-2.4.2-11 positive and negative) | `internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go` | AC-2: labels 0, 15 refused with the reserved-range reason; 16 and 1048575 accepted and read back exactly from bytes 4..6 | to write, must fail first on 0 and 15 |
| `TestSRPolicyLabelWidthRefused` (new, untagged per Q4) | `internal/component/bgp/plugins/nlri/srpolicy/config_test.go` | AC-2: `binding-sid mpls 1048576`, `type-a mpls 1048576`, `type-a mpls 4294967295` refused; `type-a mpls 0` and `1048575` accepted and read back exactly | to write, must fail first (today accepted and truncated) |
| `TestSRPolicySyntaxRuleVerdicts` (new) | `internal/component/bgp/plugins/nlri/srpolicy/split_test.go` | the registered rule per AFI: 12/24 well-formed; 13, 24-under-AFI-1, 12-under-AFI-2, 25 skippable-malformed; 0, 100 bits, overrun unwalkable; two well-formed NLRIs back to back well-formed | to write |
| `TestNLRISyntaxRegistry` (new) | the new registry package's `_test.go` | register, lookup by family, duplicate registration refused, unknown family answers "no rule" | to write |
| `TestValidateMPNLRISyntaxConsultsRegistry` (new) | `internal/component/bgp/message/rfc7606_test.go` | a family with a registered rule gets its verdict mapped to treat-as-withdraw / session reset; a family without one still returns nil (unicast and BGP-LS paths unchanged) | to write |
| `TestRFC9830NLRILengthReceiveVerdicts` (new; tags RFC9830-4.2.1-2 and -4.2.1-3 positive and negative, and RFC9830-5-1, -5-2, -5-4 if Q1 = a) | `internal/component/bgp/reactor/rfc9830_nlri_receive_test.go` | AC-3/AC-4/AC-6 through `processMessage`: exact RFC 7606 action per case, Established kept for treat-as-withdraw, NOTIFICATION 3/10 for unwalkable, accepted bytes unchanged for 12/24 | to write, must fail first |
| `TestRFC9830SRPolicyWithoutTunnelEncapIsMalformed` (new; tags RFC9830-4.2.1-7 and RFC9830-4.2.1-6 positive and negative; the 4.2.1-6 row's `{single-polarity: positive}` annotation, which rests on the transmit path, is replaced because the receive path has an absent-attribute case to observe) | `internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go` | AC-5: AFI 1 and AFI 2 MP_REACH with NLRI and no attribute 23 is treat-as-withdraw; the same with one type-15 TLV has no action; MP_REACH with no NLRI and no attribute 23 has no action | to write, must fail first |
| `TestSRPolicyMalformedWithdrawalSynthesis` (new, only if Q2 = a) | `internal/component/bgp/reactor/rfc9830_nlri_receive_test.go` | the synthesized MP_UNREACH for AC-3 carries the well-formed NLRIs and none of the malformed ones | to write |

### Boundary Tests (numeric inputs)
| Field | Range | Last valid | First invalid below | First invalid above |
|-------|-------|------------|---------------------|---------------------|
| Binding SID MPLS label | 16..1048575 | 16 and 1048575 | 15 | 1048576 |
| Type A MPLS label | 0..1048575 | 0 and 1048575 | N/A (unsigned) | 1048576 |
| SR Policy NLRI length, AFI 1 | exactly 96 bits | 96 | 88 | 104 |
| SR Policy NLRI length, AFI 2 | exactly 192 bits | 192 | 184 | 200 |

### RFC-tagged tests that change (owner approval needed, Q7)
| Test | Current tags | Change | Why |
|------|--------------|--------|-----|
| `internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830TunnelTypeIsSRPolicy` | RFC9830-4.2.1-7 positive and negative, RFC9830-4.2.1-6 positive | remove both 4.2.1-7 tags AND the 4.2.1-6 positive tag; the test body is unchanged and stays as an untagged transmit guard | the audit note calls its negative conditional ("when ParseTunnelEncap errors on the bare payload it asserts nothing") and it proves transmit, not the receive rule. Section 4.2.1 is receive validation ("When a BGP speaker receives an SR Policy NLRI from a neighbor, it MUST first perform validation based on the following rules ..."), and the 4.2.1-6 line rests on the same transmit evidence (RFC-tagged test review, 2026-10-13; confirmed: the body builds and parses ze's own attribute). The new reactor test `TestRFC9830SRPolicyWithoutTunnelEncapIsMalformed` carries 4.2.1-7 and 4.2.1-6, both polarities. Its discrimination records go with the tags |
| `internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NLRICarriesAllThreeFields` | RFC9830-4.2.1-2 positive and negative | remove both 4.2.1-2 tags in this approval; the body is unchanged and stays as an untagged encoder/config/`Parse` guard. Alternative the owner may pick instead: narrow the claim to "ze's encoder, config and `Parse` hold all three fields", which still does not prove the receive rule | OVER-CLAIM (mis-scoped) at HEAD, confirmed: the body proves the encoder, the config parser and `Parse`, while the session receive path frames NLRI through `SplitSRPolicy`, which accepts any non-zero byte-aligned length (the D3 defect). The same receive-rule objection as 4.2.1-7 applies; the new `TestRFC9830NLRILengthReceiveVerdicts` carries 4.2.1-2 |
| `internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go::TestRFC9830TunnelTypeReceiveVerdicts` | RFC9830-2.2-1, -2.2-3 | none; the new absent-attribute cases go in a new function so this tagged unit and its records stay unchanged | the producer `applyTunnelEncap` changes, so `./le rfc check` may report its records `producer-changed`; renewal by `./le rfc discriminate-record` is owed, not a test change |
| tagged units whose producer changes: every RFC9830 / RFC9012 tag over `applyTunnelEncap` (`rfc9830_tunnel_receive_test.go`, `rfc9012_tunnel_encap_carry_test.go`) and over `parseConfigRoute` / `validateMPNLRISyntax` (`rfc9830_test.go`, `rfc7606_bgpls_nlri_test.go`) | as recorded | none to the tests; discrimination records renewed if `./le rfc check` reports them stale | parent R-6 / R-C1 in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` |

Untagged test that changes: `internal/component/bgp/reactor/session_prefix_receive_test.go::TestReceivePrefixLimitShortAddPathEntry`
(R-3). It keeps its assertion (an incomplete ADD-PATH entry never enters the
installed inventory); only the way it reaches the inventory changes. Not an
RFC-tagged test, so no owner approval, but it MUST NOT lose that assertion.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `srpolicy-binding-sid-label-range` | `test/parse/srpolicy-binding-sid-label-range.ci` | config with `binding-sid mpls 15`, then `1048576`, then `type-a mpls 1048576`: each load refused, exit non-zero, stderr names token, value and range | to write |
| `bgp-srpolicy-3-encode` | `test/encode/bgp-srpolicy-3-encode.ci` | API sends `binding-sid mpls 1048575` and `type-a mpls 1048575`; ze-peer expects exact UPDATE hex with `FF FF F0 00` in both label entries | to write |
| `srpolicy-receive-nlri-length` | `test/plugin/srpolicy-receive-nlri-length.ci` | ze-peer sends a well-formed policy then an UPDATE carrying one 12-octet and one 13-octet NLRI; ze logs the RFC 9830 Section 5 reason and keeps the session (no NOTIFICATION in the peer's expectations); if Q1 = a, a second connection sends a 100-bit length and expects NOTIFICATION 3/10 hex | to write |
| `srpolicy-receive-no-tunnel-encap` | `test/plugin/srpolicy-receive-no-tunnel-encap.ci` | ze-peer sends a SAFI 73 UPDATE with RT community and no attribute 23; ze logs the RFC 9830 Section 4.2.1 reason; session kept | to write |
| existing `bgp-srpolicy-1-encode`, `bgp-srpolicy-2-encode`, `bgp-srpolicy-1-decode`, `bgp-srpolicy-2-decode`, `exabgp-compat/encoding/conf-sr-policy` | `test/encode/`, `test/decode/`, `test/exabgp-compat/` | unchanged, must stay green (labels 24000, 16001, 16002 are in range) | existing |

### Interop Tests (Scope: protocol)
| Scenario | Peer daemon | What it asserts | Status |
|----------|-------------|-----------------|--------|
| `bgp-sr-policy-gobgp` (new; `test/interop/scenarios/bgp-sr-policy-gobgp/` with `ze.conf` and `gobgp.toml`; checker in `internal/le/interoplab/bgp/check_extras.go`, name constant in `names.go`) | GoBGP v3.31.0 (`test/interop/Dockerfile.gobgp`) | Ze originates an IPv4 and an IPv6 SR Policy with `binding-sid mpls 1048575` and segments `type-a mpls 16` and `type-a mpls 1048575`, with an IPv4-address Route Target; `gobgp global rib -a ipv4-srpolicy -j` (and `ipv6-srpolicy`) shows the session up, both NLRIs with their distinguisher/color/endpoint, and the Binding SID and segment labels exactly 1048575 and 16. Proves another implementation reads Ze's 20-bit labels and NLRI lengths as sent | to write |

Interop covers origination only. GoBGP's CLI cannot send a malformed SAFI 73
NLRI or omit attribute 23 on demand, so the receive side (AC-3, AC-5, AC-6)
is proven at the daemon boundary by the `.ci` tests above, where ze-peer
writes the exact bytes. Red-phase proof per `interop-and-goal-validation.md`:
revert the D2 range check and rebuild; the interop label assertion for 1048575
still passes (it is in range), so the discriminating red for D2 is the `.ci`
config refusal, and for the interop scenario it is reverting the
`buildSegmentTypeA` shift to a narrower mask, which must turn the GoBGP label
assertion red.

### Goal Validation (planned evidence)
| Goal | Evidence that will prove it |
|------|----------------------------|
| Ze never sends a label it was not configured with | `bgp-srpolicy-3-encode.ci` exact hex, plus `bgp-sr-policy-gobgp` reading 1048575 back from GoBGP |
| Ze never sends a reserved BSID label | `srpolicy-binding-sid-label-range.ci` refusal through the real config loader |
| A malformed SR Policy NLRI does not enter the RIB or reach peers as a route | `srpolicy-receive-nlri-length.ci` over a live session, `TestRFC9830NLRILengthReceiveVerdicts` exact actions |
| An SR Policy without its Tunnel Encapsulation attribute is not accepted | `srpolicy-receive-no-tunnel-encap.ci` over a live session |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | GoBGP v3.31.0 negotiates and displays `ipv4-srpolicy` / `ipv6-srpolicy` with BSID and segment labels in `-j` output | GoBGP's documented SR Policy family support; not yet run against Ze | Interop must use FRR instead, whose SAFI 73 support in bgpd is also unverified here | first run of the scenario's GoBGP side with a static policy, before writing the checker | unvalidated |
| A-2 | A treat-as-withdraw result recorded by `validateMPNLRIField` reaches the same synthesis and dispatch path as an attribute-level one | `ValidateUpdateRFC7606` records non-reset results through `recordError` (rfc7606.go ~447) | D3 needs reactor plumbing | `TestRFC9830NLRILengthReceiveVerdicts` asserts the delivered body is the synthesized withdrawal | unvalidated |
| A-3 | No live config or test uses a Binding SID label 0..15 or any label above 1048575 | grep over `internal`, `test`, `docs`, `etc` for `binding-sid mpls` / `type-a mpls` found only 24000, 16001, 16002, 1048575 | an existing test would go red on refusal | the same grep re-run at implementation | validated 2026-10-13 by grep |
| A-4 | Every reactor fixture that sends SAFI 73 MP_REACH already carries attribute 23 and 12/24-octet NLRIs, except `TestReceivePrefixLimitShortAddPathEntry` | grep of reactor tests for `SAFISRPolicy`: `teSRPolicyBody`, `teSRPolicyValue` users carry type 15 and 96/192-bit lengths | more fixtures change | package test run of `internal/component/bgp/reactor` after the change | unvalidated |
| A-5 | ADD-PATH is never negotiated for SAFI 73 in practice; the syntax rule ignores it, as `SplitSRPolicy` does | `split.go` comment "ADD-PATH is not used with SR-Policy"; RFC 9830 defines no path identifier | a peer with ADD-PATH on SAFI 73 would be read as malformed and treat-as-withdraw | R-4 test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A peer that sends SR Policy without attribute 23 (or with a non-standard length) now loses those routes | operator reports policies missing after upgrade; RFC 9830 log line | intended by RFC 9830 Sections 4.2.1 and 5; the log line names the rule so it is diagnosable |
| R-2 | Treat-as-withdraw synthesis relays the malformed NLRI bytes downstream in MP_UNREACH, which a strict downstream peer could reset its session over | downstream NOTIFICATION after an upstream malformed UPDATE | Q2 (a) filters them; if (b) is chosen this stays an accepted risk recorded here |
| R-3 | `TestReceivePrefixLimitShortAddPathEntry` stops reaching the inventory boundary, so its guard loses coverage silently if the fixture is just deleted | the test goes red on "receive changed UPDATE" | rewrite it to reach the inventory walk with the same incomplete entry by another route (direct call of the inventory walk on the same bytes, or another family whose splitter accepts the bytes); never delete the assertion |
| R-4 | ADD-PATH negotiated for SAFI 73 makes every received policy read as malformed | treat-as-withdraw on every SR Policy UPDATE from that peer | rule behaviour matches the splitter's; a unit case with `addPath=true` pins it; record a journal row if a peer is found doing it |
| R-5 | The new registry and the BGP-LS hard-coded branch disagree (two mechanisms) | a reviewer flags layering | Q6; if (a), one journal row names the BGP-LS branch for a later migration |
| R-6 | Changing `applyTunnelEncap` and `validateMPNLRISyntax` stales discrimination records of other tagged units | `./le rfc check` reports `producer-changed` | renew with `./le rfc discriminate-record` in the same change (parent R-6) |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Received SR Policies dropped (treat-as-withdraw) or a session reset on a mis-judged length; configs with labels now out of range refused at load |
| How is it reverted? | single commit revert; no config migration (no in-range config changes meaning) |
| Who else touches this path? | `spec-bgp-update-propagation-rfc-defects` (receive path, `session_validation.go`), `plan/spec-bgp-tunnel-encapsulation-consumption.md` (attribute 23), `spec-rfc-verdict-fix-bgp` (re-judges after this lands) |

## Files to Modify
- `internal/component/bgp/plugins/nlri/srpolicy/config.go` - range checks in `parseConfigRoute` at `binding-sid mpls` and `type-a mpls`
- `internal/component/bgp/plugins/nlri/srpolicy/split.go` - the per-AFI syntax rule beside `SplitSRPolicy`, sharing its framing walk
- `internal/component/bgp/plugins/nlri/srpolicy/register.go` - register the rule for both families
- `internal/component/bgp/message/rfc7606.go` - `validateMPNLRISyntax` consults the registry for families with no built-in rule, mapping skippable to treat-as-withdraw and unwalkable to session reset
- `internal/component/bgp/reactor/session_tunnel_encap.go` - absent-attribute branch for SAFI 73 MP_REACH with NLRI
- `internal/component/bgp/message/rfc7606_withdraw.go` - only if Q2 = a
- `internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go`, `config_test.go`, `split_test.go`, `internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go`, `session_prefix_receive_test.go`, `internal/component/bgp/message/rfc7606_test.go`
- `rfc/short/rfc9830.md`, `rfc/discrimination/rfc9830.json`, `rfc/audit/rfc9830.json` (through `./le rfc` tools only), `docs/features/rfc-status.md` (regenerated)
- `docs/architecture/wire/nlri.md`, `docs/architecture/wire/attributes.md`, `docs/architecture/wire/messages.md`, `docs/features/bgp-protocol.md`; `docs/architecture/core-design.md` only if Q2 = a

## Files to Create
- the registry package in `internal/core/bgp/nlri/` (name chosen at implementation, beside `nlrisplit` and `nlritype`) and its test
- `internal/component/bgp/reactor/rfc9830_nlri_receive_test.go`
- `test/parse/srpolicy-binding-sid-label-range.ci`, `test/encode/bgp-srpolicy-3-encode.ci`, `test/plugin/srpolicy-receive-nlri-length.ci`, `test/plugin/srpolicy-receive-no-tunnel-encap.ci`
- `test/interop/scenarios/bgp-sr-policy-gobgp/ze.conf`, `gobgp.toml`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | SR Policy route content is a token string parsed by `parseConfigRoute`, not YANG leaves |
| YANG validation constraints | N-A | same: the range lives in the parser, the only place the label is read |
| YANG custom validators | N-A | no YANG node |
| CLI commands/flags | N-A | no new command |
| CLI grammar (keyword before value) | N-A | existing `binding-sid mpls <label>` grammar unchanged |
| Editor autocomplete | N-A | no YANG node |
| Functional test for new RPC/API | Yes | `test/encode/bgp-srpolicy-3-encode.ci` drives the API route path |
| Pipe completeness | N-A | no output command |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency added |
| Prometheus counters/metrics | No | existing RFC 7606 diagnostics carry the event; no new counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | SAFI 73 exists; no new family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | SR Policy receive validation and label range: `docs/features/bgp-protocol.md`; `./le feature report` checked for an SR Policy feature id |
| 2 | Config syntax changed? | No | syntax unchanged; only accepted range narrows, documented in `docs/features/bgp-protocol.md` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | same tokens; out-of-range values now refused (documented with row 1) |
| 5 | Plugin added/changed? | No | the srpolicy plugin gains a registration, no user-visible plugin change |
| 6 | Has a user guide page? | No | `docs/guide/bgp-peering.md` mentions the family only |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/nlri.md` (receive length rule), `docs/architecture/wire/attributes.md` (absent attribute on SAFI 73) |
| 8 | Plugin SDK/protocol changed? | No | in-process registry only |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9830.md` rows and `Support remaining`; `docs/features/rfc-status.md` regenerated |
| 10 | Test infrastructure changed? | No | new tests only |
| 11 | Affects daemon comparison? | No | no SR Policy claim in `docs/comparison.md` changes |
| 12 | Internal architecture changed? | Yes | the new registry: `docs/architecture/wire/nlri.md` section on NLRI receive validation |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/immediate/spec-bgp-sr-policy-rfc-defects.md` at implementation; known: `split.go` declares `docs/architecture/wire/nlri.md` (affected, row 7); `session_tunnel_encap.go` declares `docs/architecture/wire/attributes.md` (affected, row 7); `docs/features/bgp-protocol.md` anchors `config.go` (affected, row 1); `rfc7606.go` declares `docs/architecture/wire/messages.md` (affected: its RFC 7606 paragraph names only IPv4 NLRI syntax for 3/10 and must add the registered MP NLRI rules and the SR Policy treat-as-withdraw case); `rfc7606_withdraw.go` declares `docs/architecture/core-design.md` (affected only if Q2 = a, which changes what synthesis carries; otherwise unaffected because synthesis is unchanged); `config.go` declares `docs/architecture/config/syntax.md` (unaffected: the page does not describe SR Policy route tokens or label ranges, checked by grep 2026-10-13) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `test/encode/bgp-srpolicy-*.ci` and `docs/features/bgp-protocol.md` examples use labels 24000/16001, in range; verify at implementation |

### Discovery (repo-maintenance Mechanical Checklist)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md` row for NLRI wire / RFC 7606, leading to `docs/architecture/wire/nlri.md`, which will name the syntax registry |
| What rule prevents regression? | `principles.md` registration directive; the registry is the one place a family's NLRI syntax rule is declared |
| What registry/inventory prevents drift? | the new per-family syntax registry, read by family only |
| What verification proves it? | `TestValidateMPNLRISyntaxConsultsRegistry` and the reactor receive tests; `./le rfc check` over the tagged rows |

## Implementation Steps

1. **Phase: Wiring** - registry package and its test; `validateMPNLRISyntax` consults it; srpolicy registers a rule that answers well-formed for everything. Tests: `TestNLRISyntaxRegistry`, `TestValidateMPNLRISyntaxConsultsRegistry`. Verify: the reactor receive test reaches the rule (it fails on the 13-octet case because the rule is a stub)
2. **Phase: D3 length rule** - write `TestSRPolicySyntaxRuleVerdicts` and `TestRFC9830NLRILengthReceiveVerdicts`, see them fail, implement the rule, then Q1 and Q2 per the owner's answers. Update `docs/architecture/wire/nlri.md` in this phase
3. **Phase: D4 absent attribute** - `TestRFC9830SRPolicyWithoutTunnelEncapIsMalformed` red, then the `applyTunnelEncap` branch; fix `TestReceivePrefixLimitShortAddPathEntry` per R-3. Update `docs/architecture/wire/attributes.md` in this phase
4. **Phase: D2 label range** - `TestRFC9830BindingSIDLabelRange`, `TestSRPolicyLabelWidthRefused` red, then the parser checks with user-facing messages per `ai/rules/user-facing-errors.md`. Update `docs/features/bgp-protocol.md` in this phase
5. **Phase: functional and interop** - the four `.ci` tests, the `bgp-sr-policy-gobgp` scenario and its forced-red run
6. **Phase: RFC records** - tag changes approved under Q7; `rfc/short/rfc9830.md` gap markers and `Support remaining`; `./le rfc discriminate-record` for every new or changed tagged unit; renew stale records; hand AC-4/AC-5 to an agent that did not write the tests for `./le rfc audit-stamp ... mode rejudge`
7. Gates owed (run by the main thread, not inside a five-minute agent): `./le go lint run`, `./le verify worktree`, `./le test integration interop` selecting `bgp-sr-policy-gobgp`

## Key Insights
- D1 is already fixed (`a936e853c7`); this spec now owns D2, D3, D4 (AC-5) and the AC-4/AC-5 verdicts
- One parser (`parseConfigRoute`) serves config, API and the ExaBGP bridge, so D2 is two checks
- SAFI 73 receive validation today is only the tunnel TLV type check; the NLRI is never judged at the session, and the RIB drops splitter errors silently
- RFC 9830 Section 5 decides the action: framed-but-wrong length is treat-as-withdraw, unwalkable framing is session reset (Ze has no AFI/SAFI disable)
- Section 5 forbids receive-side semantic checks of sub-TLV fields; all D2 checks are origination-only

## RFC Documentation (Scope: protocol)
Add `// RFC 9830 Section X.Y: "<quoted requirement>"` above each new check, and
the section reference at each caller, per `ai/rules/rfc-compliance.md` (functions
added after 2026-09-24). The syntax rule carries the Section 2.1 NLRI diagram
with field offsets.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated (AC-6 only if Q1 = a)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`

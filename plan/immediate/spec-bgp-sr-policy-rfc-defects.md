# Spec: bgp-sr-policy-rfc-defects

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 5/6 |
| Handoff | - |
| Updated | 2026-10-14 |

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
is complete, and every owner question is answered ("Owner decisions": Q7 on
2026-10-13, Q1 to Q6 on 2026-10-14).

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 9830 Section 2.4.2 | "The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt." | `internal/component/bgp/plugins/nlri/srpolicy/config.go::buildBindingSIDSubTLV` | Writes Flags 0x10, an unassigned bit (only S 0x80 and I 0x40 are defined) | no row names it |
| D2 | RFC 9830 Section 2.4.2 (Figure 6), with RFC 3032 Section 2.1 for the label width (no row) | RFC 9830: "If the length is 6, then the BSID is encoded in 4 octets using the format below." and "The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15)." Figure 6 gives Label the first 20 bits, and RFC 3032 Section 2.1 fixes the width: "This 20-bit field carries the actual value of the Label." | `internal/component/bgp/plugins/nlri/srpolicy/config.go` `binding-sid mpls` parse (`strconv.ParseUint` to 32 bits) and `buildBindingSIDSubTLV` (`label << 4`, written through `byte()`); the same width shape in the `type-a mpls` parse and `buildSegmentTypeA` (RFC 9830 Section 2.4.4.2.1: "Label:  20 bits of label value.") | A configured label above 1048575 is accepted and its high bits are lost in the byte truncation, so the peer receives a different label with no error. A Binding SID label 0 to 15 is accepted and sent, against the MUST NOT. A Type A segment label wider than 20 bits truncates the same way | no row; progress DEFECT line; no journal row |
| D3 | RFC 9830 Section 4.2.1 (RFC9830-4.2.1-2) | "The SR Policy NLRI MUST include a distinguisher, Color, and Endpoint field that implies that the length of the NLRI MUST be either 12 or 24 octets (depending on the address family of the Endpoint)." | `internal/component/bgp/plugins/nlri/srpolicy/split.go::SplitSRPolicy` | Frames any non-zero byte-aligned length; an NLRI of another length is accepted on receipt | weak; row has no `{gap}` marker (4.2.1-3 shares the producer) |

### Status at HEAD (re-read 2026-10-13, HEAD `a8e502fd51`)

| ID | Open at HEAD? | Evidence |
|----|---------------|----------|
| D1 | No, fixed in `a936e853c7` ("rfc: land discriminated runtime proofs and current judgments", 2026-10-06) | `buildBindingSIDSubTLV` no longer writes `buf[2] = 0x10`; the zeroed buffer leaves Flags clear and the function carries the Section 2.4.2 quote. `rfc/audit/rfc9830.json` records RFC9830-2.4.2-6 and -2.4.2-5 `enforced` over `rfc9830_transmit_zero_test.go::TestRFC9830FieldsToIgnoreAreZeroOnTransmission`; `test/encode/bgp-srpolicy-1-encode.ci` expects Binding SID bytes `0D 06 00 00 05 DC 00 00` (Flags 0x00). The BGP child spec-rfc-verdict-fix-bgp (closed 2026-10-21; the table survives in git history) listed 2.4.2-6 under "Historical transfers ... enforced". Nothing remains for D1 here |
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
| Interop | an inject scenario where Ze receives the malformed bytes and GoBGP downstream shows the result, with recorded revert-reds, plus an origination scenario against GoBGP (see Interop Tests) |

## Owner decisions

Every question below is answered. Q7 was answered by the owner on 2026-10-13.
Q1 to Q6 were answered on 2026-10-14: "Do what you think is right", which
accepts the recommendation on every open owner question. Each row records the
option chosen, and the ACs, tests, files and risks below carry only the chosen
branch.

| Row | Question | Options | Recommendation |
|-----|----------|---------|----------------|
| Q1 | D3 scope: RFC 9830 Section 5 also requires session reset for NLRI framing Ze cannot walk (zero length, non-byte-aligned length, an NLRI overrunning the attribute): "Alternately, the router MUST perform "session reset" when the session is only being used for SR Policy or when a "AFI/SAFI disable" action is not possible." Ze has no AFI/SAFI disable (`internal/component/bgp/message/rfc7606_bgpls_nlri.go` says so for BGP-LS). Today such framing is accepted at the session and silently dropped by the RIB. These are the RFC9830-5-1 and -5-4 gap rows, not rows this spec was handed | (a) include: the same registered syntax rule answers both verdicts (AC-6); (b) leave as recorded gaps | (a): one rule, one producer, and leaving it out keeps a silent drop on the same path this spec edits. **Owner, 2026-10-14: accept (recommendation), option (a): session reset for NLRI framing Ze cannot walk is in this spec (AC-6).** |
| Q2 | Treat-as-withdraw synthesis (`message.SynthesizeWithdrawFamilies`) turns MP_REACH into MP_UNREACH carrying the SAME NLRI bytes and relays it to every eligible peer. For D3 that relays a 13-octet NLRI Ze has just judged malformed | (a) the registered SR Policy rule also drops the malformed NLRIs at the existing `typedNLRIEdit` pass, which `applyRFC7606` already runs under treat-as-withdraw ("It is NOT skipped for treat-as-withdraw") and whose output is the body synthesis reads ("The body handed on is the one the Section 5.4 filter above left"); BGP-LS already drops its malformed NLRIs there through `RetainWellFormedNLRI`. `SynthesizeWithdrawFamilies` is unchanged; the withdrawal then carries only the well-formed NLRIs (nothing in any Adj-RIB-In can be keyed by the malformed ones once AC-3 holds). When the retain answer drops every NLRI the UPDATE carries, `typedNLRIEdit` returns `typedNLRIEmptied` and `applyRFC7606` takes that case: treat-as-withdraw with nothing synthesized and nothing relayed, which is correct because no route was ever keyed by those NLRIs; (b) relay unchanged, as every other family does | (a): no change to synthesis, only the rule's retain answer at a pass that exists. Under (b), R-2 is concrete, not hypothetical: a downstream Ze that receives the relayed MP_UNREACH-only UPDATE reads the malformed NLRI under the same rule and resets that session (see "Verdict per MP_UNREACH case"). **Owner, 2026-10-14: accept (recommendation), option (a): the registered SR Policy rule drops the malformed NLRI bytes at `typedNLRIEdit`; synthesis unchanged.** |
| Q3 | RFC9830-4.2.1-4 and -4.2.1-5 (the update MUST carry NO_ADVERTISE or an IPv4-address Route Target, else malformed) and -4.2.1-8 sit in the same Section 4.2.1 list and would use the same receive hook. They are recorded `{gap}` rows and not in this spec | (a) keep out, they stay gaps; (b) add as AC-7 | (a): scope is the owner's, and (b) changes what every SR Policy peer must send. Under (a) the -4.2.1-8, -5-8 and -4.2.1-1 markers still change: their text says no receive criterion is evaluated, already untrue at HEAD (`applyTunnelEncap` checks type 15), and after this spec only the Route Target / NO_ADVERTISE criterion remains, so each marker narrows to that criterion (AC-7). **Owner, 2026-10-14: accept (recommendation), option (a): RFC9830-4.2.1-4, -4.2.1-5 and -4.2.1-8 stay out as gaps; the -4.2.1-8, -5-8 and -4.2.1-1 markers narrow to the Route Target / NO_ADVERTISE criterion.** |
| Q4 | D2 width: RFC 9830 states the 20-bit width only as a field description ("Label:  20 bits of label value.", Section 2.4.4.2.1; Figure 6 for the BSID; RFC 3032 Section 2.1 "This 20-bit field carries the actual value of the Label."). There is no MUST, so no requirement row can carry it, and RFC scope is frozen against new enrolment | (a) no new row: the width refusal is proven by untagged regression tests, since a silently truncated value is a `principles.md` defect independent of any row; (b) add a row to `rfc/short/rfc9830.md` | (a). **Owner, 2026-10-14: accept (recommendation), option (a): no new row; the 20-bit width refusal is proven by untagged regression tests.** |
| Q5 | D2 reserved range on Type A segments: RFC 9830 forbids labels 0-15 only for the Binding SID (Section 2.4.2). Section 2.4.4.2.1 places no such rule on a Type A segment | (a) Type A keeps accepting 0-15, only the width is checked; (b) refuse 0-15 there too | (a): refusing it would be a rule the RFC does not state. **Owner, 2026-10-14: accept (recommendation), option (a): Type A segments keep accepting labels 0-15; only the width is checked.** |
| Q6 | The new per-family NLRI syntax registry (Design) would sit beside two hard-coded family rules at the same receive gate: BGP-LS, which `validateMPNLRISyntax` and `typedNLRIEdit` both reach through `NLRISyntaxRuled`, and EVPN, which `typedNLRIEdit` names by literal (`isEVPN`, `ValidEVPNNLRILengths`) | (a) leave BGP-LS and EVPN where they are and record the hard-coded branches as a journal row; (b) one registry replaces `NLRISyntaxRuled` at all three of its call sites and the EVPN literal: the `typedNLRIEdit` gate (`reactor/session_validation_nlritype.go`), `validateMPNLRISyntax` (`message/rfc7606.go`), and the family guard inside `RetainWellFormedNLRI` itself (`message/rfc7606_bgpls_nlri.go`, which passes any family `NLRISyntaxRuled` does not name through unchanged). BGP-LS, EVPN and SR Policy each register their section verdict and their retain answer. `RetainWellFormedNLRI` becomes BGP-LS's registered retain answer, called only through the registry: its family guard goes, and it keeps the SAFI only for the BGP-LS / BGP-LS-VPN distinction it already draws. Its untagged test `TestRetainWellFormedNLRILeavesOtherFamiliesAlone` (`message/rfc7606_bgpls_nlri_test.go`) calls it with IPv4 unicast and expects that guard's uncopied pass-through; that case no longer reaches the function, so its assertion moves to the registry lookup: the same IPv4 unicast section, malformed as a Link-State NLRI, finds no registered answer and passes through uncopied (`TestNLRISyntaxRegistry` and `TestTypedNLRIEditConsultsRegistry`), and the BGP-LS test keeps its BGP-LS cases. No RFC-tagged test body changes: that file carries no `RFC requirement:` tag; the BGP-LS tagged tests (`message/rfc9552_bgpls_test.go`, `reactor/rfc9552_*_test.go`) drive the receive path, so `./le rfc check` may report their records `producer-changed`, renewed by `./le rfc discriminate-record` (no owner approval, no body edit) | (b): (a) leaves two mechanisms for one question at one gate, which `ai/rules/no-layering.md` forbids, and keeps R-5; (b) is one registry read by family at every site, so R-5 goes away. Cost, stated in full: the BGP-LS and EVPN producers move into registrations; `RetainWellFormedNLRI` loses its family guard and changes from a function every family passes through to BGP-LS's registered answer; one untagged BGP-LS test case moves to the registry; the reactor gate, the validator and three wire pages (`nlri.md`, `nlri-bgpls.md`, `nlri-evpn.md`) change; and every BGP-LS and EVPN discrimination record the change stales is renewed. **Owner, 2026-10-14: accept (recommendation), option (b): one NLRI-syntax registry for BGP-LS, EVPN and SR Policy, replacing `NLRISyntaxRuled` at its three sites and the EVPN literal (`ai/rules/no-layering.md`).** |
| Q7 | RFC-tagged test changes listed in the TDD Test Plan, "RFC-tagged tests that change": `TestRFC9830TunnelTypeIsSRPolicy` loses its RFC9830-4.2.1-7 (both) and RFC9830-4.2.1-6 (positive) tags; `TestRFC9830NLRICarriesAllThreeFields` loses its RFC9830-4.2.1-2 tags (both), or has them narrowed if the owner prefers. Both rest on transmit/encoder evidence for Section 4.2.1, which opens "When a BGP speaker receives an SR Policy NLRI from a neighbor, it MUST first perform validation based on the following rules ..."; the new receive tests carry the rows (revised 2026-10-13 after the RFC-tagged test review) | approve or refuse each row | approve. **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** Both rows are approved as listed in the TDD plan; the owner's words pick no narrowing alternative, so the listed edit (removing the 4.2.1-2 tags) stands. Commands under "Owner approval commands" in the TDD plan |

D2 has no row of its own in `rfc/short/rfc9830.md` (Q4).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/attributes.md` (the SR Policy carrier paragraph, the one stating carriers "require exactly one type-15 TLV")
  → Constraint: the page says actual SR Policy carriers "require exactly one type-15 TLV"; it is silent on an absent attribute. AC-5 makes the page wrong unless the same change states that a SAFI 73 MP_REACH with NLRI and no attribute 23 is treat-as-withdraw
- [ ] `docs/architecture/wire/nlri.md` (family table, SR Policy entry; `split.go` declares it as its `// Design:` page)
  → Constraint: the page names `SplitSRPolicy` as the framing walk; it must gain the receive length rule (96 bits for AFI 1, 192 for AFI 2) and its two verdicts in the same change
- [ ] `docs/features/bgp-protocol.md`, the Binding SID flags note ("SR-Policy route encoding leaves the Binding SID's unassigned flag bits clear", anchored to `buildBindingSIDSubTLV`)
  → Constraint: this is where the operator-visible label range belongs: Binding SID 16 to 1048575, Type A 0 to 1048575
- [ ] `docs/architecture/testing/interop.md`
  → Decision: BGP scenarios live in `test/interop/scenarios/<name>/` with a checker registered in `internal/le/interoplab/bgp/checkers.go` / `check_extras.go`; GoBGP checks read `gobgp global rib -a <family> -j` (pattern: `bgp-evpn-gobgp`). No SR Policy scenario exists at HEAD
  → Decision: the optional `inject.msg` sidecar (`le test peer`, raw injector at 172.30.0.9) exists to "Drive Ze with wire bytes no conforming daemon would emit"; worked examples `bgp-rfc7606-relay-shape-frr` and `bgp-rfc7606-typed-nlri-discard`. The receive side of this spec is proven against GoBGP through it (Interop Tests)
  → Constraint: before relying on a scenario, revert the fix, rebuild the image, see it red, restore, see it green, and record the red; the vacuity traps in the same page apply

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9830.md`, `rfc/full/rfc9830.txt` Sections 2.1, 2.4.2, 2.4.4.2.1, 4.2.1 and 5
  → Constraint: Section 5 fixes the action per error class. Well-framed but invalid: "When the error determined allows for the router to skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message, then it MUST handle such malformed NLRIs as 'treat-as- withdraw'." Unwalkable: "Alternately, the router MUST perform "session reset" when the session is only being used for SR Policy or when a "AFI/SAFI disable" action is not possible." Ze has no AFI/SAFI disable, so the second always means session reset
  → Constraint: Section 2.1 ties the length to the AFI, not to the endpoint: "When AFI = 1, the value MUST be 96; when AFI = 2, the value MUST be 192." So a 24-octet body under AFI 1 is malformed even though 4.2.1 lists 24 as a legal length
  → Constraint: Section 5 forbids receive-side field checks: "A BGP implementation MUST NOT perform semantic verification of such fields nor consider the SR Policy update to be invalid or not usable based on such validation." The D2 label checks run on Ze's own origination only, never on receipt
  → Constraint: Section 2.4.2: "The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15)." binds the originator; Section 2.4.4.2.1 says only "Label:  20 bits of label value." for Type A
- [ ] `rfc/full/rfc3032.txt` Section 2.1: "This 20-bit field carries the actual value of the Label."
  → Constraint: any configured label above 1048575 has no encoding and MUST be refused, not truncated
- [ ] `rfc/full/rfc7606.txt` Section 2: treat-as-withdraw means the UPDATE "MUST be treated as though all contained routes had been withdrawn"
  → Decision: an invalid SR Policy NLRI withdraws every route of its UPDATE, not only the bad NLRI. RFC 9830 Section 5's skippable sentence ("skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message ... handle such malformed NLRIs as 'treat-as- withdraw'") can be read per NLRI; the whole-UPDATE reading rests on Section 5 "An SR Policy update that is determined not to be valid (and, therefore, malformed) based on the rules described in Section 4.2.1 MUST be handled by the "treat-as-withdraw" strategy." together with the first Section 4.2.1 rule, which makes a wrong NLRI length an invalid update ("The SR Policy NLRI MUST include a distinguisher, Color, and Endpoint field that implies that the length of the NLRI MUST be either 12 or 24 octets ...")
- [ ] `rfc/full/rfc7606.txt` Section 5.2: "Thus, if an UPDATE message is encountered that does contain path attributes other than MP_UNREACH_NLRI and doesn't encode any reachable NLRI, we cannot be confident that the NLRI have been successfully parsed as Section 3 (j) requires. For this reason, if any path attribute errors are encountered in such an UPDATE message and if any encountered error specifies an error-handling approach other than "attribute discard", then the "session reset" approach MUST be used."
  → Constraint: Ze's escalation in `ValidateUpdateRFC7606AddPath` (`!hasReachableNLRI && strongest > RFC7606ActionAttributeDiscard`) omits the "other than MP_UNREACH_NLRI" condition, so an UPDATE whose only attribute is MP_UNREACH_NLRI is escalated too. Pre-existing deviation, recorded as a journal row in `plan/journal/gate-fires-outside-its-population.md` (2026-10-14) and NOT fixed by this spec; the verdicts it produces are stated in "Verdict per MP_UNREACH case" and in Known Limitations
- [ ] `rfc/full/rfc4271.txt` Section 6.3 and `rfc/full/rfc4760.txt` Section 7 (NOTIFICATION subcode for an MP NLRI reset)
  → Decision: AC-6 expects NOTIFICATION 3/1 (Malformed Attribute List), not 3/10. RFC 4271 Section 6.3 assigns Invalid Network Field to the legacy field only: "The NLRI field in the UPDATE message is checked for syntactic validity.  If the field is syntactically incorrect, then the Error Subcode MUST be set to Invalid Network Field." An SR Policy NLRI travels inside MP_REACH_NLRI or MP_UNREACH_NLRI, an optional attribute, not in that field. RFC 4760 Section 7 names a different subcode for an incorrect MP attribute ("The session SHOULD be terminated with the Notification message code/subcode indicating "UPDATE Message Error"/"Optional Attribute Error"."), and RFC 7606 Section 3 (a) leaves the subcode to the error ("The error subcode elaborates on the specific nature of the error."). Ze's every MP NLRI reset today (MP unicast through `ValidateNLRISyntaxAddPath`, BGP-LS through `validateBGPLSNLRISyntax`) sets no `Notification`, so `applyRFC7606` sends it through `rfc7606SessionReset` as 3/1; the SR Policy verdict follows the same route, so one MP NLRI reset has one subcode. The 3/9 SHOULD is recorded in Known Limitations, not changed here

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/config.go` (502L) - `parseConfigRoute` parses every route token and parses `binding-sid mpls` itself; `type-a mpls` is parsed in `parseSegmentList`, which `parseConfigRoute` calls. Both use `strconv.ParseUint(..., 10, 32)` with no range check; `buildBindingSIDSubTLV` and `buildSegmentTypeA` write `label << 4` into bytes 4..6 through `byte()`, so bits above 20 are lost. Flags already zero (D1 fixed)
  → Constraint: two entry points reach `parseConfigRoute`: config `nlri { ipv4/sr-policy add ... }` through `registry.ConfigRouteParserByFamily` (its one non-test caller is `bgp/config/bgp_routes.go`), and `ze bgp encode -f ipv4/sr-policy` through `registry.RouteEncoderByFamily` and `EncodeRoute` (its one non-test caller is `bgp/cli/encode.go`). The API does NOT: `send bgp ... update text ... nlri ipv4/sr-policy` goes to `parseSRPolicySection` (`plugins/cmd/update/update_text_srpolicy.go`, chosen by the SAFI test in `update_text_nlri.go`), which accepts only `add`/`del`, `distinguisher`, `color` and `endpoint`, and refuses every other token with `sr-policy: unknown keyword %q; valid: distinguisher, color, endpoint`. So no SR Policy attribute, and no label, can be sent over the API at HEAD, and the D2 range check is never reached from it (Known Limitations). The ExaBGP bridge (`internal/exabgp/bridge/bridge_command.go::convertAnnounceSRPolicy`) emits that API text with its tunnel-encap tokens appended, so it meets the same refusal (journal row in `plan/journal/parser-refuses-a-form-its-producer-emits.md`, 2026-10-14). The D2 checks therefore cover config and `ze bgp encode`, the two paths that reach them
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/split.go` (51L) - `SplitSRPolicy` refuses zero length, non-byte-aligned length and overrun; accepts any other length; ignores ADD-PATH (its `nlrisplit.Splitter` signature receives `addPath` and discards it as `_ bool`; the doc comment claims "ADD-PATH is not used with SR-Policy")
  → Constraint (I-8, verified at the producer): production config CAN negotiate ADD-PATH for SAFI 73. `parseAddPathFromTree` (`internal/component/bgp/reactor/config_capabilities.go`) applies a default `add-path { direction ... }` to every `capability.Multiprotocol` the peer carries, with no family exclusion, and accepts any `family.LookupFamily` name per family; the YANG `add-path` description says "The default direction applies to every negotiated multiprotocol family". RFC 9830 does not forbid a path identifier. So the rule MUST honour `addPath` (skip the 4-octet path identifier before the length octet), as `RetainWellFormedNLRI` does for BGP-LS ("reading its first two octets as an NLRI Type would condemn well-formed NLRIs"). The rule and `SplitSRPolicy` share one framing walk, so the splitter honours `addPath` in the same change and its "not used" comment goes (`ai/rules/stale-comments.md`)
  → Constraint: the splitter is shared by the RIB walk, diagnostics (`mpDiagnosticNLRI`) and forwarding; making it stricter changes every consumer and the RIB discards its error (`internal/component/bgp/plugins/rib/rib_structured.go`), so strictness in the splitter alone is a silent drop, not an RFC 7606 action
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/types.go` (144L) - `Parse` refuses a body shorter than 12/24 but reads a longer one, ignoring the tail
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/register.go` (45L) - `init()` registers both families' splitter with `nlrisplit.Register` and the plugin registration; no `nlritype` recognizer
  → Decision: the new syntax rule is registered here, beside the splitter, so no core file learns SR Policy
- [ ] `internal/component/bgp/message/rfc7606.go` - `ValidateUpdateRFC7606AddPath` (which `ValidateUpdateRFC7606` wraps) calls `validateMPNLRIField` for MP_REACH and MP_UNREACH inside its attribute walk; a session-reset result returns at once, any other result is recorded through `recordError` and becomes the UPDATE's verdict. After the walk, `!hasReachableNLRI && strongest > RFC7606ActionAttributeDiscard` escalates any such verdict to session reset (no MP_UNREACH-only exemption; see RFC 7606 Section 5.2 above). `validateMPNLRISyntax` handles BGP-LS through the hard-coded `NLRISyntaxRuled`, IPv4/IPv6 unicast and multicast through the prefix walk, and returns nil for every other family, SAFI 73 included. None of its verdicts sets `Notification`
  → Constraint: a treat-as-withdraw verdict from this function already reaches the session's synthesis path when the UPDATE has reachable NLRI; the new rule needs no new reactor plumbing for D3
- [ ] `internal/component/bgp/reactor/session_validation_receive.go` - `classifyRFC7606` sets `syntaxField` only for the legacy Withdrawn Routes and NLRI fields
- [ ] `internal/component/bgp/reactor/session_validation.go` - `applyRFC7606` sends a `syntaxField` result to `rfc7606NLRISyntaxAction` (NOTIFICATION 3/10, RFC 4271 Section 6.3), and every other session-reset result to `rfc7606ResetNotification` with the result's own `Notification`, or to `rfc7606SessionReset` (NOTIFICATION 3/1, Malformed Attribute List) when it carries none. An MP NLRI verdict is an attribute-level result, so it never reaches `rfc7606NLRISyntaxAction`: it leaves as 3/1. Inside `applyRFC7606`, `applyTunnelEncap` runs after duplicate stripping and before `applyTypedNLRIDiscard`, which runs under treat-as-withdraw too ("It is NOT skipped for treat-as-withdraw"), and the body it leaves is what synthesis reads ("The body handed on is the one the Section 5.4 filter above left")
  → Decision: AC-6 expects 3/1 (see the RFC 4271 / RFC 4760 entry under Required Reading)
- [ ] `internal/component/bgp/reactor/session_validation_nlritype.go` - `typedNLRIEdit` leaves before splitting any family with no `nlritype` recognizer, not EVPN (`isEVPN`, then `message.ValidEVPNNLRILengths`) and not `NLRISyntaxRuled` (BGP-LS, then `message.RetainWellFormedNLRI`), which is why SAFI 73 framing errors reach no verdict today. Two hard-coded family rules sit at this gate (Q6)
- [ ] `internal/component/bgp/reactor/session_tunnel_encap.go` (263L) - `applyTunnelEncap` returns at once when `TunnelEncapPresent` is false; when present it sets `policy` for AFI 1/2 SAFI 73 and refuses a non-15 tunnel type or a second type-15 TLV with `tunnelReceiveError` (treat-as-withdraw, escalated to session reset when the UPDATE has no reachable NLRI)
  → Decision: the AC-5 absent-attribute check goes in this function, beside the existing SAFI 73 carrier test, so the type-15 rule and the presence rule share one site. That test (the `policy` flag) sits after the `if !result.TunnelEncapPresent` early return, so the D4 branch MUST come before that return: placed after it, it never runs for the one case it exists for
- [ ] `internal/component/bgp/message/rfc7606_withdraw.go` - `SynthesizeWithdrawFamilies` copies the MP_REACH NLRI bytes into MP_UNREACH unchanged; under Q2 (a) it is not edited, because the malformed NLRIs are already gone from the body it reads (`typedNLRIEdit`)
- [ ] `internal/component/bgp/reactor/session_prefix_receive_test.go::TestReceivePrefixLimitShortAddPathEntry` - builds a session with a negotiated ADD-PATH receive context for ipv4/sr-policy (`prefixReceiveSession(..., true)`) and sends `08 00` as the third argument of `prefixReceiveBody`, which goes to `mpUnreachAttrs`: an MP_UNREACH-only UPDATE, no other attribute, no attribute 23. It asserts the receive succeeds and that the incomplete ADD-PATH entry never enters the installed inventory
  → Constraint: AC-5 does not touch it (no MP_REACH). With the rule honouring ADD-PATH (I-8), `08 00` is two of the four path-identifier octets, so the section is unwalkable. Under Q1 (a) the UPDATE is a session reset with NOTIFICATION 3/1 (AC-6), it no longer reaches the inventory, and the fixture moves (R-3). (A rule that ignored ADD-PATH would read `08 00` as a framed 1-octet NLRI, skippable-malformed, and the Section 5.2 escalation would turn it into a session reset too)

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
- Origination: config `nlri { ipv4/sr-policy add ... }` and `ze bgp encode -f ipv4/sr-policy "..."`; both reach `parseConfigRoute`. The API and the ExaBGP bridge reach `parseSRPolicySection` instead, which carries no SR Policy attribute (Current Behavior, Known Limitations)
- Receipt: an UPDATE from a peer carrying MP_REACH_NLRI or MP_UNREACH_NLRI with AFI 1 or 2 and SAFI 73

### Transformation Path
1. Origination: route tokens, then `parseConfigRoute` (D2 range checks land here, before any byte is built), then `buildTunnelEncap` and the sub-TLV builders, then the UPDATE builder
2. Receipt: `Session.classifyRFC7606`, then `ValidateUpdateRFC7606AddPath`, then `validateMPNLRIField` and `validateMPNLRISyntax`, which consults the per-family syntax registry with the negotiated `addPath` (D3: the srpolicy rule answers well-formed, malformed-skippable or unwalkable); after the attribute walk, the Section 5.2 escalation turns a non-discard verdict on an UPDATE with no reachable NLRI into session reset
3. Receipt, session side (`applyRFC7606`): a session-reset verdict from an MP attribute carries no `Notification`, so it leaves through `rfc7606SessionReset` as NOTIFICATION 3/1; any other verdict continues; duplicate stripping; `applyTunnelEncap` (D4: SAFI 73 MP_REACH with NLRI and no attribute 23 becomes treat-as-withdraw); `applyTypedNLRIDiscard` through `typedNLRIEdit`, which asks the registered rule (Q2 (a)) to drop the malformed SR Policy NLRIs; treat-as-withdraw synthesis over the body that pass left; RIB

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| srpolicy plugin to the BGP receive validator | the plugin registers its NLRI syntax rule per family in `init()`; `message` reads the registry by family, never by name | No (wiring tests below) |
| config and `ze bgp encode` to srpolicy plugin | `InProcessConfigRouteParser` / `InProcessRouteEncoder` registrations, unchanged | Yes (existing `.ci` encode tests) |

### Integration Points
- `nlrisplit` registry (unchanged) and the new per-family NLRI syntax registry (core, beside `nlrisplit`)
- `applyTunnelEncap`'s existing SAFI 73 carrier test (the D4 branch goes before its `TunnelEncapPresent` early return)
- `typedNLRIEdit`'s per-family discard pass: the rule's retain answer (Q2 (a)), and the registry replaces `NLRISyntaxRuled` and the EVPN literal there (Q6 (b))

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | D3 enters through `validateMPNLRISyntax`, the function that already owns MP NLRI syntax verdicts; D4 through `applyTunnelEncap`, which already owns SAFI 73 tunnel verdicts |
| No unintended coupling | Yes | `message` learns no SR Policy layout; the rule lives in the srpolicy plugin |
| No duplicated functionality | Yes (Q6 (b)) | the registry replaces `NLRISyntaxRuled` at its three sites (`validateMPNLRISyntax`, `typedNLRIEdit`, the guard inside `RetainWellFormedNLRI`) and the EVPN literal, so one mechanism answers the question |
| Zero-copy preserved | Yes | the rule walks the received slice; it allocates nothing on a well-formed UPDATE |
| Registration over hardcoding, outbound | Yes | the srpolicy plugin registers its rule in `register.go` |
| Registration over hardcoding, inbound | Partly | searched `validateMPNLRISyntax`, `NLRISyntaxRuled`, `typedNLRIEdit`, `nlritype`: SAFI 73 is named in none of them after this change. `applyTunnelEncap` already names SAFI 73 by literal (pre-existing, `policy` flag); D4 extends that site rather than adding a second. The BGP-LS (`NLRISyntaxRuled`) and EVPN (`isEVPN`) literals go: both families register in the same registry (Q6 (b)) |

## Design

### Alternatives for D3 (receive NLRI length)

| Approach | How | Gains | Costs |
|----------|-----|-------|-------|
| A (chosen) | A per-family NLRI syntax registry in core; `validateMPNLRISyntax` asks it by family; srpolicy registers a rule per AFI that walks the section (skipping the 4-octet path identifier when `addPath` is set) and returns well-formed, skippable-malformed (framed, wrong length for the AFI) or unwalkable (zero, non-aligned, overrun, truncated path identifier), plus a retain answer that `typedNLRIEdit` uses (Q2 (a)) | The verdict reaches the existing RFC 7606 action path; no core file names SR Policy; Q1 (a) comes for free | One new small registry. It replaces `NLRISyntaxRuled` at its three call sites and the EVPN literal, so BGP-LS and EVPN register too (Q6 (b)) |
| B | Add a SAFI 73 case to `validateMPNLRISyntax` in `message` | Smallest diff | A central enumeration (`principles.md`), and `message` would carry the SR Policy layout |
| C | Register an `nlritype` recognizer for SAFI 73 that returns false on a wrong length | Reuses Section 5.4 machinery; framing errors already reach session reset there | Its action is NLRI discard, not the treat-as-withdraw RFC 9830 Section 5 requires, and it misnames a length rule as an unrecognized type |
| D | Make `SplitSRPolicy` refuse a wrong length | One-line change | The RIB discards the error, so the route vanishes silently; no RFC 7606 action, no log (`principles.md`: silently wrong) |

### Alternatives for D2 (label range)

| Approach | How | Gains | Costs |
|----------|-----|-------|-------|
| A (recommended) | Range checks at the two label tokens: `binding-sid mpls` in `parseConfigRoute` (BSID 16..1048575) and `type-a mpls` in `parseSegmentList` (Type A 0..1048575), refused with a message naming the token, the value and the range | Both entry points that carry a label (config, `ze bgp encode`) share the one parser; nothing reaches the builders out of range | none found |
| B | Mask in the builders | No refusal path | Still sends a different label than configured, which is the defect |

### Proposed changes

| Area | Current | Proposed |
|------|---------|----------|
| `binding-sid mpls <n>` | any 32-bit value; high bits truncated; 0..15 sent | 16..1048575 accepted unchanged; 0..15 refused citing the reserved range; above 1048575 refused citing the 20-bit width |
| `type-a mpls <n>` | any 32-bit value, truncated | 0..1048575 accepted; above refused (Q5: 0..15 stays accepted) |
| received SAFI 73 NLRI, framed, length not 96 bits (AFI 1) / 192 bits (AFI 2) | carried to RIB and peers | the whole UPDATE is treat-as-withdraw, logged with the RFC 9830 Section 5 reason; the session stays up |
| received SAFI 73 NLRI, unwalkable (zero, non-aligned, overrun, a path identifier cut short under ADD-PATH) | carried, then dropped silently by the RIB | session reset with NOTIFICATION 3/1 Malformed Attribute List (Q1; subcode per the RFC 4271 / RFC 4760 entry in Required Reading) |
| received SAFI 73 NLRI under negotiated ADD-PATH | `SplitSRPolicy` discards `addPath` and reads the path identifier's first octet as the length octet, so it mis-frames every entry; no session verdict | the rule and `SplitSRPolicy` skip the 4-octet path identifier, then judge the length (I-8) |
| received SAFI 73 MP_REACH with NLRI and no attribute 23 | accepted | treat-as-withdraw, reason names RFC 9830 Section 4.2.1 |
| received SAFI 73 MP_REACH with no NLRI and no attribute 23 | accepted | unchanged: it carries no SR Policy NLRI, so it is not an SR Policy update |
| received SAFI 73 MP_UNREACH only | accepted | unchanged for attribute 23; the length rule applies to its NLRIs too, with the verdicts in the next table |

### Verdict per MP_UNREACH case (skippable-malformed SR Policy NLRI in MP_UNREACH_NLRI)

The Section 5.2 escalation in `ValidateUpdateRFC7606AddPath` stays as it is (journal row, Known Limitations), so the verdicts are:

| UPDATE shape | Reachable NLRI? | Verdict | Matches RFC 7606 Section 5.2? |
|--------------|-----------------|---------|-------------------------------|
| MP_UNREACH_NLRI is the only path attribute | no | session reset, NOTIFICATION 3/1 (escalated) | no: the RFC exempts this shape ("other than MP_UNREACH_NLRI"), so treat-as-withdraw is owed. Pre-existing deviation, not fixed here |
| MP_UNREACH_NLRI plus other attributes (ORIGIN, AS_PATH, ...), no MP_REACH NLRI, no legacy NLRI | no | session reset, NOTIFICATION 3/1 (escalated) | yes |
| MP_UNREACH_NLRI beside an MP_REACH_NLRI (any family) or legacy NLRI that carries reachable NLRI | yes | treat-as-withdraw, session stays Established | yes |

An unwalkable MP_UNREACH section (Q1 (a)) is a session reset in every shape, before the escalation is reached.

### Failure modes considered
Recorded as R-1 to R-7 below.

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
| config `nlri { ipv4/sr-policy add ... binding-sid mpls 1048575 ... type-a mpls 1048575 }` | → | `parseConfigRoute`, `buildBindingSIDSubTLV`, `buildSegmentTypeA` | `test/encode/bgp-srpolicy-3-encode.ci` (config-driven, as `bgp-srpolicy-1-encode.ci` and `-2-encode.ci` are; exact hex of label 0xFFFFF in both sub-TLVs) |
| peer sends SAFI 73 UPDATE with a 13-octet NLRI | → | registered srpolicy syntax rule via `validateMPNLRISyntax` | `test/plugin/srpolicy-receive-nlri-length.ci` and `internal/component/bgp/reactor/rfc9830_nlri_receive_test.go::TestRFC9830NLRILengthReceiveVerdicts` |
| peer sends SAFI 73 UPDATE without attribute 23 | → | `applyTunnelEncap` absent-attribute branch | `test/plugin/srpolicy-receive-no-tunnel-encap.ci` and `internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go::TestRFC9830SRPolicyWithoutTunnelEncapIsMalformed` |
| `ze show bgp encode -f ipv4/sr-policy "... binding-sid mpls 15 ..."` (and `1048576`, `type-a mpls 1048576`; then one in-range route) | → | `parseConfigRoute` / `parseSegmentList` range checks through `registry.RouteEncoderByFamily` and `EncodeRoute` | `test/parse/cli-bgp-encode-srpolicy-label-range.ci` (pattern: `test/parse/flowspec-action-encoding.ci`): each out-of-range command exits 1 with stderr naming token, value and range; the in-range command exits 0 with the exact label hex on stdout, so the refusals are not vacuous |
| peer with ADD-PATH receive negotiated for ipv4/sr-policy sends a 12-octet NLRI behind a path identifier | → | registered srpolicy rule honouring `addPath` | `internal/component/bgp/reactor/rfc9830_nlri_receive_test.go::TestRFC9830NLRILengthReceiveVerdicts` ADD-PATH cases |
| raw injector sends a 13-octet NLRI and a SAFI 73 UPDATE without attribute 23 to Ze; GoBGP is Ze's downstream peer | → | registered srpolicy rule, `applyTunnelEncap` branch, `typedNLRIEdit` (Q2 a), synthesis | `bgp-sr-policy-malformed-receive-gobgp` interop scenario |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | any announced SR Policy with a Binding SID | Flags octet carries only defined bits. Already met at HEAD by `a936e853c7` (RFC9830-2.4.2-6 `enforced`); kept so the preservation tests stay named, no new work |
| AC-2 | `binding-sid mpls 1048576`, `binding-sid mpls 15`, `binding-sid mpls 0`, `type-a mpls 1048576`, through config (`test/parse/srpolicy-binding-sid-label-range.ci`) and through `ze bgp encode` (`test/parse/cli-bgp-encode-srpolicy-label-range.ci`), the two entry points that reach `parseConfigRoute` (the API does not, see Known Limitations) | each refused before any UPDATE is built; through config the load fails, through `ze bgp encode` the command exits 1 and prints no hex; the message names the token, the value and the allowed range; `binding-sid mpls 16`, `binding-sid mpls 1048575`, `type-a mpls 0`, `type-a mpls 1048575` accepted and encoded with exactly that 20-bit label in bytes 4..6 and TC/S/TTL zero |
| AC-3 | received SAFI 73 NLRI whose length octet is byte-aligned, fits the attribute, and is not 96 for AFI 1 or 192 for AFI 2 (cases: AFI 1 with 13 octets, AFI 1 with 24 octets, AFI 2 with 12 octets, AFI 2 with 25 octets), in MP_REACH, and in MP_UNREACH beside reachable NLRI | the UPDATE is treat-as-withdraw: every route it carries is withdrawn from the Adj-RIB-In, the session stays Established, no NOTIFICATION is sent, and the log line names RFC 9830 Section 5 and the offending length. The same malformed NLRI in an UPDATE with no reachable NLRI is a session reset with NOTIFICATION 3/1, in both shapes of "Verdict per MP_UNREACH case" (MP_UNREACH alone, and MP_UNREACH with other attributes); the test asserts that verdict as the current escalation's and names the journal row, it does not claim it conformant for the MP_UNREACH-alone shape. 12-octet AFI 1 and 24-octet AFI 2 NLRIs, alone and beside each other's families in separate UPDATEs, are accepted with no RFC 7606 action. The synthesized withdrawal carries only the well-formed NLRIs of the UPDATE and none of the malformed ones (Q2 (a)); when every NLRI is malformed, nothing is synthesized or relayed (`typedNLRIEmptied`) |
| AC-4 | the `weak` verdict of RFC9830-4.2.1-2 (RFC9830-2.4.2-6 is already `enforced`, see Status at HEAD) | a tagged receive-side test proves the quoted sentence in both polarities, `./le rfc discriminate-record` records its break, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge` to `enforced`. Moved here from "Blocked by" in the BGP child spec-rfc-verdict-fix-bgp, closed 2026-10-21 (parent P-3, 2026-09-28) |
| AC-5 | the `weak` verdict of RFC9830-4.2.1-7 ("The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message and MUST have a Tunnel Type TLV set to SR Policy (code point is 15)."), a Section 4.2.1 receive-validation rule (D4) | a received SAFI 73 UPDATE whose MP_REACH carries NLRI and which has no attribute 23 is treat-as-withdraw, logged with the RFC 9830 Section 4.2.1 reason, session stays up; with exactly one type-15 TLV it is accepted with no action; a non-15 type or two type-15 TLVs stay treat-as-withdraw as today. A tagged test proves it in both polarities, `./le rfc discriminate-record` records its break, and an agent that did not write that test re-judges the verdict with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in the BGP child spec-rfc-verdict-fix-bgp, closed 2026-10-21 (main-thread ruling, 2026-09-28) |
| AC-6 | received SAFI 73 NLRI section that cannot be walked: length octet 0, length not a multiple of 8, or an NLRI overrunning the attribute (Q1 (a)) | session reset with NOTIFICATION Update Message Error / Malformed Attribute List (3/1), the subcode every MP NLRI reset carries today (`rfc7606SessionReset`; reason in Required Reading, RFC 4271 Section 6.3 assigns 3/10 to the legacy NLRI field only), logged with the RFC 9830 Section 5 reason; RFC9830-5-4 loses its `{gap}` marker and carries a tagged test |
| AC-7 | `rfc/short/rfc9830.md` after the fix, row by row | RFC9830-2.4.2-11: marker removed, tagged both polarities (AC-2). RFC9830-4.2.1-3: marker removed, tagged both polarities (AC-3). RFC9830-4.2.1-6: `{single-polarity: positive}` annotation removed, tagged both polarities on the receive path (AC-5, Q7). RFC9830-5-2: marker narrowed, unconditionally (it is the skippable case and does not depend on Q1), to the one shape Ze still resets instead of treating as withdrawn: a malformed NLRI in an UPDATE whose only attribute is MP_UNREACH_NLRI, citing the journal row; tagged both polarities over the reachable-NLRI shapes (AC-3). RFC9830-5-1: marker removed (length per NLRI, total MP attribute length, and length-to-AFI consistency all judged; AC-3, AC-6; Q1 (a)). RFC9830-5-4: marker removed (AC-6; Q1 (a)). RFC9830-2.1-2 ("When AFI = 1, the value MUST be 96; when AFI = 2, the value MUST be 192."): unchanged (tagged by `TestRFC9830NLRIAddressFamilies`: encode positive on `EncodeNLRIHex`, decode and split negative on `Parse` and `SplitSRPolicy`; `SplitSRPolicy` changes under AC-8, so that record may report `producer-changed` and is renewed under the "tagged units whose producer changes" row); the AFI-length half of AC-3 exercises it on receipt untagged, and no receive tag is added. RFC9830-4.2.1-1, -4.2.1-8 and -5-8: markers narrowed to the one remaining Section 4.2.1 criterion, the Route Target / NO_ADVERTISE rule (Q3 (a); RFC9830-4.2.1-4 and -4.2.1-5 unchanged), naming the criteria now judged (NLRI length, attribute 23 present with exactly one type-15 TLV). The `Support remaining` sentence no longer lists "Reserved MPLS binding-label validation" and no longer says complete SAFI 73 NLRI validation is not established for the length rule. Every `file:line` in a touched marker is replaced by file plus symbol. `docs/features/rfc-status.md` regenerated from it |
| AC-8 | ADD-PATH receive negotiated for ipv4/sr-policy and ipv6/sr-policy (production config `add-path { direction receive; }`) | a well-formed NLRI behind its 4-octet path identifier is accepted with no RFC 7606 action and `SplitSRPolicy` frames it as one entry; a 13-octet NLRI behind a path identifier is treat-as-withdraw; a path identifier cut short is unwalkable (session reset with NOTIFICATION 3/1, Q1 (a)); without ADD-PATH the same bytes are judged without the skip |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures an SR Policy with `binding-sid mpls 15` | config load, `parseConfigRoute`, refusal | `srpolicy-binding-sid-label-range.ci` |
| 2 | encodes an SR Policy with `type-a mpls 1048576` through `ze bgp encode` | CLI, `registry.RouteEncoderByFamily`, `EncodeRoute`, `parseConfigRoute`, `parseSegmentList`, exit 1 with the refusal on stderr | `cli-bgp-encode-srpolicy-label-range.ci` |
| 3 | announces an SR Policy with labels 1048575 and 16 | config: builders, UPDATE, GoBGP RIB; `ze bgp encode`: builders, hex on stdout | config: `bgp-srpolicy-3-encode.ci`, `bgp-sr-policy-gobgp`; CLI: `cli-bgp-encode-srpolicy-label-range.ci` (exact label hex of the in-range route) |
| 4 | receives a malformed SR Policy from a peer and relays to a downstream peer | wire, `ValidateUpdateRFC7606AddPath`, registry rule, `applyRFC7606`, `typedNLRIEdit`, synthesis, downstream | `srpolicy-receive-nlri-length.ci`, `bgp-sr-policy-malformed-receive-gobgp` |
| 5 | receives an SR Policy with no Tunnel Encapsulation attribute | wire, `applyTunnelEncap` branch, treat-as-withdraw, downstream | `srpolicy-receive-no-tunnel-encap.ci`, `bgp-sr-policy-malformed-receive-gobgp` |
| 6 | peers with ADD-PATH receive on ipv4/sr-policy | negotiated receive context, rule and splitter skip the path identifier | `TestRFC9830NLRILengthReceiveVerdicts` ADD-PATH cases |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9830BindingSIDLabelRange` (new; tags RFC9830-2.4.2-11 positive and negative) | `internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go` | AC-2: labels 0, 15 refused with the reserved-range reason; 16 and 1048575 accepted and read back exactly from bytes 4..6 | to write, must fail first on 0 and 15 |
| `TestSRPolicyLabelWidthRefused` (new, untagged per Q4) | `internal/component/bgp/plugins/nlri/srpolicy/config_test.go` | AC-2: `binding-sid mpls 1048576`, `type-a mpls 1048576`, `type-a mpls 4294967295` refused; `type-a mpls 0` and `1048575` accepted and read back exactly | to write, must fail first (today accepted and truncated) |
| `TestSRPolicySyntaxRuleVerdicts` (new) | `internal/component/bgp/plugins/nlri/srpolicy/split_test.go` | the registered rule per AFI: 12/24 well-formed; 13, 24-under-AFI-1, 12-under-AFI-2, 25 skippable-malformed; 0, 100 bits, overrun unwalkable; two well-formed NLRIs back to back well-formed; with `addPath` set, the same cases behind a 4-octet path identifier give the same verdicts and a path identifier cut short is unwalkable; the retain answer keeps exactly the well-formed NLRIs; `SplitSRPolicy` with `addPath` frames one entry per path identifier plus NLRI (AC-8) | to write, the ADD-PATH cases must fail first against today's splitter |
| `TestNLRISyntaxRegistry` (new) | the new registry package's `_test.go` | register, lookup by family, duplicate registration refused, unknown family answers "no rule"; also the case moved from `TestRetainWellFormedNLRILeavesOtherFamiliesAlone`: an IPv4 unicast section malformed as a Link-State NLRI finds no registered answer and is left uncopied | to write |
| `TestValidateMPNLRISyntaxConsultsRegistry` (new) | `internal/component/bgp/message/rfc7606_test.go` | a family with a registered rule gets its verdict mapped to treat-as-withdraw / session reset with no `Notification` set; a family without one still returns nil (unicast path unchanged); BGP-LS reaches its rule through the registry with the verdicts it has today | to write |
| `TestTypedNLRIEditConsultsRegistry` (new, Q6 (b)) | `internal/component/bgp/reactor/session_validation_nlritype_test.go` | `typedNLRIEdit` reads the registry, not `NLRISyntaxRuled` or the EVPN literal: BGP-LS malformed NLRIs dropped and EVPN invalid lengths refused exactly as today; a family with no rule and no recognizer leaves before locating the attribute, its section unchanged | to write |
| `TestRFC9830NLRILengthReceiveVerdicts` (new; tags RFC9830-4.2.1-2, -4.2.1-3 and -5-2 positive and negative unconditionally, RFC9830-5-1 and RFC9830-5-4 per AC-7) | `internal/component/bgp/reactor/rfc9830_nlri_receive_test.go` | AC-3/AC-4/AC-6/AC-8 through `processMessage`: exact RFC 7606 action per case, Established kept for treat-as-withdraw, NOTIFICATION 3/1 for unwalkable, accepted bytes unchanged for 12/24; the three "Verdict per MP_UNREACH case" shapes each asserted (session reset 3/1, session reset 3/1, treat-as-withdraw), the MP_UNREACH-alone case untagged and commented with the journal row; ADD-PATH cases with a negotiated receive context | to write, must fail first |
| `TestRFC9830SRPolicyWithoutTunnelEncapIsMalformed` (new; tags RFC9830-4.2.1-7 and RFC9830-4.2.1-6 positive and negative; the 4.2.1-6 row's `{single-polarity: positive}` annotation, which rests on the transmit path, is replaced because the receive path has an absent-attribute case to observe) | `internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go` | AC-5: AFI 1 and AFI 2 MP_REACH with NLRI and no attribute 23 is treat-as-withdraw; the same with one type-15 TLV has no action; MP_REACH with no NLRI and no attribute 23 has no action | to write, must fail first |
| `TestSRPolicyMalformedWithdrawalSynthesis` (new, Q2 (a)) | `internal/component/bgp/reactor/rfc9830_nlri_receive_test.go` | through `processMessage`, the body after `typedNLRIEdit` and the synthesized MP_UNREACH for AC-3 carry the well-formed NLRIs and none of the malformed ones; `SynthesizeWithdrawFamilies` itself is not edited | to write |

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
| tagged units whose producer changes: every RFC9830 / RFC9012 tag over `applyTunnelEncap` (`rfc9830_tunnel_receive_test.go`, `rfc9012_tunnel_encap_carry_test.go`), over `parseConfigRoute` / `parseSegmentList` (`srpolicy/rfc9830_test.go`), and the receive-path tagged tests over `validateMPNLRISyntax` / `typedNLRIEdit` (the BGP-LS ones in `message/rfc9552_bgpls_test.go` and `reactor/rfc9552_*_test.go`; `message/rfc7606_bgpls_nlri_test.go` carries no `RFC requirement:` tag, so it has no record to renew) | as recorded | none to the tests; discrimination records renewed if `./le rfc check` reports them `producer-changed` | R-6 in `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` |

Untagged test that changes (Q1 (a)): `internal/component/bgp/reactor/session_prefix_receive_test.go::TestReceivePrefixLimitShortAddPathEntry`
(R-3). Its MP_UNREACH-only `08 00` becomes a session reset (Current Behavior).
It keeps its assertion (an incomplete ADD-PATH entry never enters the
installed inventory); only the way it reaches the inventory changes. Not an
RFC-tagged test, so no owner approval, but it MUST NOT lose that assertion.

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q7). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`; both names were checked to exist at HEAD `051876c3f4`. `TestRFC9830TunnelTypeReceiveVerdicts` and the producer-changed units keep their text, so the gate does not ask for them; the new receive tests are new units and need none.

| # | Command (run as written, one per prompt) |
|---|------------------------------------------|
| 1 | `./le rfc approve unit srpolicy.TestRFC9830TunnelTypeIsSRPolicy reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-sr-policy-rfc-defects, RFC9830-4.2.1-7 tags and the 4.2.1-6 positive tag removed)'` |
| 2 | `./le rfc approve unit srpolicy.TestRFC9830NLRICarriesAllThreeFields reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-sr-policy-rfc-defects, RFC9830-4.2.1-2 tags removed)'` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `srpolicy-binding-sid-label-range` | `test/parse/srpolicy-binding-sid-label-range.ci` | config with `binding-sid mpls 15`, then `1048576`, then `type-a mpls 1048576`: each load refused, exit non-zero, stderr names token, value and range | to write |
| `bgp-srpolicy-3-encode` | `test/encode/bgp-srpolicy-3-encode.ci` | config-driven, like `bgp-srpolicy-1-encode.ci`: the config block originates `binding-sid mpls 1048575` and `type-a mpls 1048575` (`ipv4/sr-policy add ...`); le test peer expects exact UPDATE hex with `FF FF F0 00` in both label entries, as the only route-carrying UPDATE; the per-family End-of-RIB markers are expected beside it (as `api-announce-flowspec-extra-token.ci` expects its EOR) | to write |
| `srpolicy-receive-nlri-length` | `test/plugin/srpolicy-receive-nlri-length.ci` | le test peer sends a well-formed policy, then an UPDATE carrying one 12-octet and one 13-octet NLRI, then on the same connection an UPDATE with a 100-bit length (Q1 (a)); ze logs the RFC 9830 Section 5 treat-as-withdraw reason for the second, no session-reset line names it, and ze answers the third with NOTIFICATION 3/1 hex (Malformed Attribute List). That NOTIFICATION fences the second UPDATE in TCP order and proves the session survived it, so one connection replaces the planned second one | written 2026-10-14, green against `bin/ze-srpolicy`; red: FAIL `output does not contain "RFC 9830 Section 5: SR Policy NLRI length 104 bits, the family requires 96"` against `bin/ze-srpolicy-red-len` (go build -overlay, `judgeSRPolicySection` guard made `if true \|\| wrongBits < 0`; the tree untouched) |
| `srpolicy-receive-no-tunnel-encap` | `test/plugin/srpolicy-receive-no-tunnel-encap.ci` | le test peer sends policy B with its tunnel, then B with an RT community and no attribute 23, then the 100-bit UPDATE as a fence; ze logs the RFC 9830 Section 4.2.1 treat-as-withdraw reason, no session-reset line names it, and the fence's NOTIFICATION 3/1 proves the session survived | written 2026-10-14, green against `bin/ze-srpolicy`; red: FAIL `output does not contain "RFC 9830 Section 4.2.1: SR Policy UPDATE carries no Tunnel Encapsulation attribute"` against `bin/ze-srpolicy-red-d4` (go build -overlay, `applyTunnelEncap` branch made `if false && srPolicyAnnounced(...)`; the tree untouched) |
| `cli-bgp-encode-srpolicy-label-range` | `test/parse/cli-bgp-encode-srpolicy-label-range.ci` | the shape of `test/parse/flowspec-action-encoding.ci`: `ze show bgp encode -f ipv4/sr-policy` with `binding-sid mpls 15`, then `binding-sid mpls 1048576`, then `type-a mpls 1048576`, each `expect=exit:code=1` with stderr naming the token, the value and the range; then one in-range route (`binding-sid mpls 1048575`, `type-a mpls 1048575`) with `expect=exit:code=0` and stdout containing each sub-TLV pinned whole, `0D060000FFFFF000` for the Binding SID and `01060000FFFFF000` for the Type A segment, so a wrong encoding of either label fails it, and so the refusals come from the range check and not from a broken command | to write, must fail first (today the out-of-range values are encoded, exit 0) |
| existing `bgp-srpolicy-1-encode`, `bgp-srpolicy-2-encode`, `bgp-srpolicy-1-decode`, `bgp-srpolicy-2-decode`, `exabgp-compat/encoding/conf-sr-policy` | `test/encode/`, `test/decode/`, `test/exabgp-compat/` | unchanged, must stay green (labels 24000, 16001, 16002 are in range) | existing |

### Interop Tests (Scope: protocol)
| Scenario | Peer daemon | What it asserts | Status |
|----------|-------------|-----------------|--------|
| `bgp-sr-policy-gobgp` (new; `test/interop/scenarios/bgp-sr-policy-gobgp/` with `ze.conf` and `gobgp.toml`; checker in `internal/le/interoplab/bgp/check_extras.go`, name constant in `names.go`) | GoBGP v3.31.0 (`test/interop/Dockerfile.gobgp`) | Ze originates an IPv4 and an IPv6 SR Policy with `binding-sid mpls 1048575` and segments `type-a mpls 16` and `type-a mpls 1048575`, with an IPv4-address Route Target; `gobgp global rib -a ipv4-srpolicy -j` (and `ipv6-srpolicy`) shows the session up, both NLRIs with their distinguisher/color/endpoint, and the Binding SID and segment labels exactly 1048575 and 16. Shows another implementation reads Ze's 20-bit labels and NLRI lengths as sent. It is goal validation for origination only: no revert of this spec's change turns it red (the asserted values are in range, so the D2 range check never fires), so it is NOT claimed as the discriminating interop test for any change here | written 2026-10-14; GREEN on Linux 2026-10-20 (65651c27e0). A-1 resolved differently from the plan: no gobgp release (v3.31.0 to v4.10.0) names SAFI 73 in its `-a` switch, and GoBGP v3.31.0 apiutil corrupts SR Policy on the way back from API form (NLRI Length divided by 8 twice, Binding SID shifted 12 more bits, so 1048575 printed 1044480). The checker runs `gobgp-paths` (`test/interop/Dockerfile.gobgp`): gRPC ListPath with `EnableOnlyBinary`, decoded by GoBGP's packet decoder, matching its `String()` output. Journal row in `plan/journal/field-carries-two-meanings.md` |
| `bgp-sr-policy-malformed-receive-gobgp` (new; `test/interop/scenarios/bgp-sr-policy-malformed-receive-gobgp/` with `ze.conf`, `gobgp.toml`, `inject.msg` and, if needed, `inject-args`; checker in `internal/le/interoplab/bgp/check_extras.go`, name constant in `names.go`) | raw injector (`le test peer`, 172.30.0.9) upstream of Ze; GoBGP v3.31.0 downstream | The injector first announces two well-formed IPv4 SR Policies (A and B) with one type-15 TLV each, then sends an UPDATE carrying A's 12-octet NLRI beside a 13-octet NLRI, then an UPDATE re-announcing B with no attribute 23. GoBGP's `gobgp global rib -a ipv4-srpolicy -j` shows A and B present after the first step and both absent after the last; the Ze-GoBGP session and the injector-Ze session stay Established throughout (no NOTIFICATION on either). The Ze-GoBGP session staying up is also the observable for "no 13-octet NLRI reaches GoBGP": GoBGP cannot decode such an NLRI, so it never appears in `-j`, and it drops the session instead (see the red-phase text). That session-up assertion rests on Q2 (a): the synthesized MP_UNREACH carries only A's well-formed NLRI, so no 13-octet NLRI reaches GoBGP in either MP attribute | written 2026-10-14; GREEN on Linux 2026-10-20 (65651c27e0), through the same `gobgp-paths` reader, which is what "GoBGP's text RIB" below now means. Both revert-reds below are still owed: on 2026-10-20 the Mac's Colima daemon refused the suite before any scenario (it applies AppArmor and cannot load `ze-kernel-probe`, whose setup is Linux-only and needs root), so they run in the QEMU guest (`./le test qemu docker-lab`) or on Linux. As written it adds a control policy C (color 400, endpoint 10.0.0.5) that nothing touches: each absence step carries C as its proof, and C still present at the end shows the injector-Ze session was never reset (the injector takes one connection, and Ze withdraws C with a dropped session). A is required absent after step 2 and B after step 3, read from GoBGP's text RIB for the reason in the row above |

Red-phase proof per `interop-and-goal-validation.md`, for
`bgp-sr-policy-malformed-receive-gobgp`: revert the registered length rule,
rebuild the Ze image, run the scenario, and see it red; restore and see green.
The expected red is the Ze-GoBGP session drop: Ze relays the 13-octet NLRI in
MP_REACH, GoBGP's `SRPolicyNLRI.decodeFromBytes` refuses any length other than
96 or 192 bits as Malformed Attribute List, `getErrorHandlingFromPathAttribute`
maps an MP_REACH or MP_UNREACH error to AFISAFI_DISABLE, and its `fsm.go` turns
that into SESSION_RESET. Then revert the D4 branch in `applyTunnelEncap`,
rebuild, and see it red; restore and see green. The expected red there is B
staying installed at GoBGP, with the session up: GoBGP's `validate.go` has no
attribute-23 check for SAFI 73, so it installs what Ze relays. Both expected
reds were read from a GoBGP v4.4.0 source checkout, not from the v3.31.0 image
the scenario runs, so they are unverified for that image and are confirmed at
implementation by the recorded red runs. Both reds are recorded in the spec. Vacuity check: each
step's absence assertion follows a presence assertion of the same policy on the
same session, so an absence caused by a dead session or a never-relayed route
fails the presence step first. The D2 origination change is discriminated by
the `.ci` refusals (config and `ze bgp encode`), not by interop.

### Goal Validation (planned evidence)
| Goal | Evidence that will prove it |
|------|----------------------------|
| Ze never sends a label it was not configured with | `bgp-srpolicy-3-encode.ci` exact hex (config), `cli-bgp-encode-srpolicy-label-range.ci` exact label hex of the in-range route (`ze bgp encode`), plus `bgp-sr-policy-gobgp` reading 1048575 back from GoBGP |
| Ze never sends a reserved BSID label | `srpolicy-binding-sid-label-range.ci` refusal through the real config loader |
| A malformed SR Policy NLRI does not enter the RIB or reach peers as a route | `bgp-sr-policy-malformed-receive-gobgp` (GoBGP downstream loses the policy, sessions stay up, recorded revert-red), `srpolicy-receive-nlri-length.ci` over a live session, `TestRFC9830NLRILengthReceiveVerdicts` exact actions |
| An SR Policy without its Tunnel Encapsulation attribute is not accepted | `bgp-sr-policy-malformed-receive-gobgp` (recorded revert-red of the D4 branch), `srpolicy-receive-no-tunnel-encap.ci` over a live session |
| An out-of-range label is refused on every entry point that carries a label | `srpolicy-binding-sid-label-range.ci` (config), `cli-bgp-encode-srpolicy-label-range.ci` (`ze bgp encode`, exit 1, no hex). The API carries no label at all (Known Limitations) |
| A conforming ADD-PATH peer's SR Policies are not misjudged | `TestRFC9830NLRILengthReceiveVerdicts` ADD-PATH cases through `processMessage` with a negotiated receive context |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | GoBGP v3.31.0 negotiates and displays `ipv4-srpolicy` / `ipv6-srpolicy` with BSID and segment labels in `-j` output | GoBGP's documented SR Policy family support; not yet run against Ze | Interop must use FRR instead, whose SAFI 73 support in bgpd is also unverified here | first run of the scenario's GoBGP side with a static policy, before writing the checker | unvalidated |
| A-2 | A treat-as-withdraw result recorded by `validateMPNLRIField` reaches the same synthesis and dispatch path as an attribute-level one, when the UPDATE carries reachable NLRI | `ValidateUpdateRFC7606AddPath` records non-reset results through `recordError` inside its attribute walk; `applyRFC7606` handles every non-`syntaxField` result as attribute-level | D3 needs reactor plumbing | `TestRFC9830NLRILengthReceiveVerdicts` asserts the delivered body is the synthesized withdrawal | confirmed 2026-10-14: `TestSRPolicyMalformedWithdrawalSynthesis` receives the dispatched synthesized MP_UNREACH through `processMessage`, with no reactor plumbing added |
| A-3 | No live config or test uses a Binding SID label 0..15 or any label above 1048575 | grep over `internal`, `test`, `docs`, `etc` for `binding-sid mpls` / `type-a mpls` found only 24000, 16001, 16002, 1048575 | an existing test would go red on refusal | the same grep re-run at implementation | validated 2026-10-13 by grep; re-run 2026-10-14 at implementation: out-of-range values occur only in the new `config_test.go` refusal cases |
| A-4 | Every reactor fixture that sends SAFI 73 MP_REACH already carries attribute 23 and 12/24-octet NLRIs, except `TestReceivePrefixLimitShortAddPathEntry` | grep of reactor tests for `SAFISRPolicy`: `teSRPolicyBody`, `teSRPolicyValue` users carry type 15 and 96/192-bit lengths | more fixtures change | package test run of `internal/component/bgp/reactor` after the change | unvalidated |
| A-5 | ADD-PATH CAN be negotiated for SAFI 73 by production config, so the rule and `SplitSRPolicy` honour `addPath` | `parseAddPathFromTree` applies the default direction to every `capability.Multiprotocol` with no family exclusion; YANG `add-path` description "The default direction applies to every negotiated multiprotocol family"; RFC 9830 does not forbid a path identifier; `TestReceivePrefixLimitShortAddPathEntry` already builds a negotiated ADD-PATH receive context for ipv4/sr-policy | (assumption replaced: the earlier "never negotiated" reading was wrong) | AC-8 tests | validated 2026-10-14 at the producer |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A peer that sends SR Policy without attribute 23 (or with a non-standard length) now loses those routes | operator reports policies missing after upgrade; RFC 9830 log line | intended by RFC 9830 Sections 4.2.1 and 5; the log line names the rule so it is diagnosable |
| R-2 | Treat-as-withdraw synthesis relays the malformed NLRI bytes downstream in MP_UNREACH. Concrete, not hypothetical: the relayed UPDATE is MP_UNREACH-only, so a downstream Ze reads the malformed NLRI under the same rule and the Section 5.2 escalation resets that downstream session | downstream NOTIFICATION after an upstream malformed UPDATE | Q2 (a), chosen by the owner 2026-10-14: the registered rule drops them at `typedNLRIEdit`, proven by `TestSRPolicyMalformedWithdrawalSynthesis` and the interop session-up assertion |
| R-3 | (Q1 (a)) `TestReceivePrefixLimitShortAddPathEntry` stops reaching the inventory boundary: its MP_UNREACH-only `08 00` under ADD-PATH becomes a session reset, so its guard loses coverage silently if the fixture is just deleted | the test goes red on the receive returning a session reset | rewrite it to reach the inventory walk with the same incomplete entry by another route (direct call of the inventory walk on the same bytes, or another family whose splitter accepts the bytes); never delete the assertion |
| R-4 | The ADD-PATH skip is wrong (offset or condition), so every SR Policy from a conforming ADD-PATH peer reads as malformed | treat-as-withdraw or session reset on every SR Policy UPDATE from an ADD-PATH peer | AC-8 cases in `TestSRPolicySyntaxRuleVerdicts` and `TestRFC9830NLRILengthReceiveVerdicts`, both polarities, with and without ADD-PATH |
| R-5 | Void: owner chose Q6 (b) on 2026-10-14, so no hard-coded BGP-LS or EVPN branch stays beside the registry. Residual check: the Critical Review "no-layering" row | - | - |
| R-6 | Changing `applyTunnelEncap` and `validateMPNLRISyntax` stales discrimination records of other tagged units | `./le rfc check` reports `producer-changed` | renew with `./le rfc discriminate-record` in the same change (parent R-6) |
| R-7 | Void: it applied only under Q1 (b); the owner chose Q1 (a) on 2026-10-14, and R-3 covers the fixture | - | - |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Received SR Policies dropped (treat-as-withdraw) or a session reset on a mis-judged length; configs with labels now out of range refused at load |
| How is it reverted? | single commit revert; no config migration (no in-range config changes meaning) |
| Who else touches this path? | `spec-bgp-update-propagation-rfc-defects` (receive path, `session_validation.go`), `plan/spec-bgp-tunnel-encapsulation-consumption.md` (attribute 23), `spec-rfc-verdict-fix-bgp` (closed 2026-10-21; it moved the AC-4 and AC-5 verdicts here, so their re-judge is this spec's) |

## Files to Modify
- `internal/component/bgp/plugins/nlri/srpolicy/config.go` - range checks at `binding-sid mpls` in `parseConfigRoute` and at `type-a mpls` in `parseSegmentList`
- `internal/component/bgp/plugins/nlri/srpolicy/split.go` - the per-AFI syntax rule and its retain answer beside `SplitSRPolicy`, sharing one framing walk that honours `addPath`; the "ADD-PATH is not used" comment removed
- `internal/component/bgp/plugins/nlri/srpolicy/register.go` - register the rule for both families
- `internal/component/bgp/message/rfc7606.go` - `validateMPNLRISyntax` consults the registry by family, mapping skippable to treat-as-withdraw and unwalkable to session reset with no `Notification` (3/1); the `NLRISyntaxRuled` branch goes (Q6 (b))
- `internal/component/bgp/reactor/session_validation_nlritype.go` - `typedNLRIEdit` asks the registered rule for its retain answer (Q2 (a)); the registry replaces `NLRISyntaxRuled` and the `isEVPN` literal there (Q6 (b))
- `internal/component/bgp/message/rfc7606_bgpls_nlri.go`, `internal/component/bgp/message/evpn_nlri.go` - Q6 (b): `NLRISyntaxRuled` deleted at all three sites, `RetainWellFormedNLRI` loses its family guard and becomes BGP-LS's registered retain answer, the BGP-LS and EVPN rules registered by family
- `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` - Q6 (b): the IPv4 unicast case of `TestRetainWellFormedNLRILeavesOtherFamiliesAlone` moves to the registry tests (its assertion kept, see Q6)
- `internal/component/bgp/reactor/session_tunnel_encap.go` - absent-attribute branch for SAFI 73 MP_REACH with NLRI, before the `TunnelEncapPresent` early return
- `internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go`, `config_test.go`, `split_test.go`, `internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go`, `session_prefix_receive_test.go` (Q1 (a), R-3), `internal/component/bgp/message/rfc7606_test.go`
- `rfc/short/rfc9830.md`, `rfc/discrimination/rfc9830.json`, `rfc/audit/rfc9830.json` (through `./le rfc` tools only), `docs/features/rfc-status.md` (regenerated); `rfc/discrimination/rfc9552.json` renewals for `producer-changed` BGP-LS units (Q6 (b))
- `docs/architecture/wire/nlri.md`, `docs/architecture/wire/attributes.md`, `docs/architecture/wire/messages.md`, `docs/features/bgp-protocol.md`, `docs/architecture/wire/nlri-bgpls.md` and `docs/architecture/wire/nlri-evpn.md` (Q6 (b); Documentation row 16). `docs/architecture/core-design.md` is not touched: synthesis is unchanged (Q2 (a))

## Files to Create
- the registry package in `internal/core/bgp/nlri/` (name chosen at implementation, beside `nlrisplit` and `nlritype`) and its test
- `internal/component/bgp/reactor/rfc9830_nlri_receive_test.go`
- `internal/component/bgp/reactor/session_validation_nlritype_test.go` (Q6 (b))
- `test/parse/srpolicy-binding-sid-label-range.ci`, `test/encode/bgp-srpolicy-3-encode.ci`, `test/plugin/srpolicy-receive-nlri-length.ci`, `test/plugin/srpolicy-receive-no-tunnel-encap.ci`, `test/parse/cli-bgp-encode-srpolicy-label-range.ci`
- `test/interop/scenarios/bgp-sr-policy-gobgp/ze.conf`, `gobgp.toml`
- `test/interop/scenarios/bgp-sr-policy-malformed-receive-gobgp/ze.conf`, `gobgp.toml`, `inject.msg` (and `inject-args` if needed)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | SR Policy route content is a token string parsed by `parseConfigRoute`, not YANG leaves |
| YANG validation constraints | N-A | same: the range lives in the parser, the only place the label is read |
| YANG custom validators | N-A | no YANG node |
| CLI commands/flags | N-A | no new command |
| CLI grammar (keyword before value) | N-A | existing `binding-sid mpls <label>` grammar unchanged |
| Editor autocomplete | N-A | no YANG node |
| Functional test for new RPC/API | N-A | no new RPC and no API behavior change: the API's SR Policy text never reaches `parseConfigRoute` (Known Limitations). The changed behavior is reached through config (`test/parse/srpolicy-binding-sid-label-range.ci`, `test/encode/bgp-srpolicy-3-encode.ci`) and `ze bgp encode` (`test/parse/cli-bgp-encode-srpolicy-label-range.ci`) |
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
| 3 | CLI command added/changed? | No | no new command; `ze bgp encode -f ipv4/sr-policy` now refuses out-of-range labels, documented with row 1 |
| 4 | API/RPC added/changed? | No | the API's SR Policy text does not reach the changed parser; its absent attributes are a Known Limitation, not changed here |
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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/immediate/spec-bgp-sr-policy-rfc-defects.md` at implementation; known: `split.go` declares `docs/architecture/wire/nlri.md` (affected, row 7); `session_tunnel_encap.go` declares `docs/architecture/wire/attributes.md` (affected, row 7); `docs/features/bgp-protocol.md` anchors `config.go` (affected, row 1); `rfc7606.go` declares `docs/architecture/wire/messages.md` (affected: its RFC 7606 paragraph names only IPv4 NLRI syntax for 3/10 and must add the registered MP NLRI rules and the SR Policy treat-as-withdraw case); `rfc7606_withdraw.go` is not edited (Q2 (a)), so its `docs/architecture/core-design.md` is unaffected through it; `session_validation_nlritype.go` also declares `docs/architecture/core-design.md` (unaffected: the page's RFC 7606 section names `classifyRFC7606` and the late attribute error, not `typedNLRIEdit` or any family rule, checked by grep 2026-10-14); `rfc7606_bgpls_nlri.go` (edited under Q6 (b)) declares `docs/architecture/wire/nlri-bgpls.md` (affected: its "Receive-Path Fault Management (RFC 9552 Section 8.2.2)" section says `validateMPNLRISyntax` routes AFI 16388 to the BGP-LS walk, which becomes a registry lookup) and `evpn_nlri.go` declares `docs/architecture/wire/nlri-evpn.md` (affected: its received Ethernet A-D section cites `ValidEVPNNLRILengths` and `typedNLRIEdit`, which then reach the length rule through the registry); `config.go` declares `docs/architecture/config/syntax.md` (unaffected: the page does not describe SR Policy route tokens or label ranges, checked by grep 2026-10-13) |
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
2. **Phase: D3 length rule and ADD-PATH** - write `TestSRPolicySyntaxRuleVerdicts` and `TestRFC9830NLRILengthReceiveVerdicts`, see them fail, implement the rule and the shared ADD-PATH-aware walk (AC-8), then the owner's answers of 2026-10-14: the unwalkable session reset (Q1 (a), AC-6), the retain answer at `typedNLRIEdit` (Q2 (a), `TestSRPolicyMalformedWithdrawalSynthesis`), and BGP-LS and EVPN moved onto the registry (Q6 (b), `TestTypedNLRIEditConsultsRegistry`, and the `nlri-bgpls.md` / `nlri-evpn.md` edits). Update `docs/architecture/wire/nlri.md` in this phase; fix `TestReceivePrefixLimitShortAddPathEntry` per R-3 here
3. **Phase: D4 absent attribute** - `TestRFC9830SRPolicyWithoutTunnelEncapIsMalformed` red, then the `applyTunnelEncap` branch before its `TunnelEncapPresent` early return. Update `docs/architecture/wire/attributes.md` in this phase
4. **Phase: D2 label range** - `TestRFC9830BindingSIDLabelRange`, `TestSRPolicyLabelWidthRefused` red, then the checks in `parseConfigRoute` and `parseSegmentList` with user-facing messages per `ai/rules/user-facing-errors.md`. Update `docs/features/bgp-protocol.md` in this phase
5. **Phase: functional and interop** - the five `.ci` tests, the `bgp-sr-policy-gobgp` and `bgp-sr-policy-malformed-receive-gobgp` scenarios, and the revert-red runs of the second (length rule, then D4 branch), each recorded
6. **Phase: RFC records** - tag changes approved under Q7; `rfc/short/rfc9830.md` markers row by row per AC-7 and `Support remaining`; `./le rfc discriminate-record` for every new or changed tagged unit; renew stale records; hand AC-4/AC-5 to an agent that did not write the tests for `./le rfc audit-stamp ... mode rejudge`
7. Gates owed (run by the main thread, not inside a five-minute agent): `./le go lint run`, `./le verify worktree`, `./le test integration interop` selecting `bgp-sr-policy-gobgp` and `bgp-sr-policy-malformed-receive-gobgp`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1..AC-8) has an implementation at a named symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Reset subcode is 3/1 on every SR Policy reset; the MP_UNREACH verdicts match the "Verdict per MP_UNREACH case" table exactly; the ADD-PATH skip is applied in the rule AND in `SplitSRPolicy`; the D4 branch sits before the `TunnelEncapPresent` return; label refusals name token, value and range |
| Naming | log lines name RFC 9830 Section 5 or Section 4.2.1 and the offending length |
| Data flow | `message` and `reactor` name no SR Policy layout; the registry is read by family only; synthesis is not edited |
| Rule: no-layering | no `NLRISyntaxRuled` (none of its three sites, the guard inside `RetainWellFormedNLRI` included) and no `isEVPN` literal remain beside the registry |
| Rule: rfc-compliance | every new check carries the RFC quote; each caller carries the section; the MP_UNREACH-alone verdict is not tagged as conformant |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| SR Policy syntax rule registered for both families | `TestSRPolicySyntaxRuleVerdicts`, `TestValidateMPNLRISyntaxConsultsRegistry` pass |
| Receive verdicts through the session | `TestRFC9830NLRILengthReceiveVerdicts`, `TestRFC9830SRPolicyWithoutTunnelEncapIsMalformed` pass |
| Label range refused on config and `ze bgp encode` | `srpolicy-binding-sid-label-range.ci`, `cli-bgp-encode-srpolicy-label-range.ci` pass |
| Interop with recorded reds | `bgp-sr-policy-malformed-receive-gobgp` green, two reds recorded in the spec |
| Summary rows | `./le rfc check` exits 0; AC-7 rows read as listed |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | every received octet the rule reads is bounds-checked against the MP attribute length; the path identifier skip cannot read past the section; a length octet of 0 cannot loop |
| Resource exhaustion | the rule walks once, allocates nothing on a well-formed section, and the retain answer allocates only when a peer sends something malformed (as `RetainWellFormedNLRI` does) |
| Peer-controlled session reset | a reset is reachable only for the RFC 9830 Section 5 unwalkable class and the existing Section 5.2 escalation; no new reset for a skippable error on an UPDATE with reachable NLRI |
| Error leakage | log lines carry peer, family and length, never the raw attribute beyond what `rfc7606Diagnostics` already records |
| Origination | out-of-range labels refused before any byte is built, on every entry point that reaches `parseConfigRoute` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, RESEARCH |
| Lint failure | Fix inline. If architectural, DESIGN |
| Functional test fails | Check the AC: wrong AC, DESIGN; correct AC, IMPLEMENT |
| GoBGP cannot display SR Policy (A-1 broken) | DESIGN: re-plan the downstream peer before writing the checker |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Key Insights
- D1 is already fixed (`a936e853c7`); this spec now owns D2, D3, D4 (AC-5) and the AC-4/AC-5 verdicts
- One parser (`parseConfigRoute`) serves config and `ze bgp encode`, so D2 is two checks; the API and the ExaBGP bridge reach `parseSRPolicySection`, which carries no SR Policy attribute (design review r3, B-1)
- SAFI 73 receive validation today is only the tunnel TLV type check; the NLRI is never judged at the session, and the RIB drops splitter errors silently
- RFC 9830 Section 5 decides the action: framed-but-wrong length is treat-as-withdraw, unwalkable framing is session reset (Ze has no AFI/SAFI disable)
- Section 5 forbids receive-side semantic checks of sub-TLV fields; all D2 checks are origination-only

## Design Insights
- An MP NLRI verdict is attribute-level in `applyRFC7606`: only the legacy fields carry `syntaxField`, so the 3/10 route is unreachable for any MP family (design review r1, I-1)
- The Section 5.2 escalation decides the MP_UNREACH verdict before any family rule does; the per-family rule cannot make an MP_UNREACH-only UPDATE treat-as-withdraw while the escalation omits the RFC's exemption (I-2)
- A pre-synthesis rewrite already exists (`typedNLRIEdit` under treat-as-withdraw), so "do not relay the malformed NLRI" is a retain answer, not a synthesis change (I-4)
- ADD-PATH is reachable for every multiprotocol family by the default direction, so no family-level rule may ignore `addPath` (I-8)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| SR Policy unwalkable reset sends 3/1 | 3/10 Invalid Network Field; 3/9 Optional Attribute Error | RFC 4271 Section 6.3 assigns 3/10 to the legacy NLRI field only; every MP NLRI reset in Ze sends 3/1 today; one subcode for one class. 3/9 (RFC 4760 Section 7 SHOULD) is recorded in Known Limitations |
| Rule and splitter share one ADD-PATH-aware walk | rule honours `addPath`, splitter keeps ignoring it | two walks of one layout would disagree on the same bytes; the splitter's `_ bool` is the D3 producer's own defect |
| Q2 (a) at `typedNLRIEdit` (owner, 2026-10-14) | family filter inside `SynthesizeWithdrawFamilies`; relay unchanged (Q2 (b)) | the pass exists and BGP-LS already uses it; synthesis stays family-blind |
| Q6 (b) one registry (owner, 2026-10-14) | registry beside `NLRISyntaxRuled` and the EVPN literal | `ai/rules/no-layering.md`; one question, one mechanism |
| Section 5.2 MP_UNREACH exemption not fixed here | fix it in this spec | it is a pre-existing RFC 7606 deviation across every family, not an SR Policy defect; journal row recorded, scope is the owner's |

## Known Limitations
- (Corrected 2026-10-21, review round 1 N-1.) RFC 7606 Section 5.2's "other than MP_UNREACH_NLRI" exemption is honoured since `99ed958ec5` (`ValidateUpdateRFC7606AddPath`, `rfc7606_mpunreach_only_test.go`): a skippable-malformed SR Policy NLRI in an UPDATE whose only attribute is MP_UNREACH_NLRI is treat-as-withdraw, and `rfc/short/rfc9830.md` RFC9830-5-2 carries no marker. Where the "Verdict per MP_UNREACH case" table, AC-3 or AC-7 still say that shape resets the session, this bullet supersedes them
- Every MP NLRI session reset sends 3/1; RFC 4760 Section 7 says the session "SHOULD be terminated with ... "Optional Attribute Error"" (3/9) for an incorrect MP attribute. Ze's existing MP unicast and BGP-LS resets do the same; the subcode is not changed here
- The Route Target / NO_ADVERTISE criterion of Section 4.2.1 (RFC9830-4.2.1-4, -4.2.1-5) stays a gap (Q3)
- (Corrected 2026-10-21, review round 1 N-1.) The API reaches `parseConfigRoute` since `af18a709d8`: the srpolicy registration sets `NLRISectionCarriesAttributes` (`srpolicy/register.go`), so an `update text ... nlri ipv4/sr-policy add` section is handed to the config parser and meets the D2 label range checks. Where Current Behavior, the Entry Point or AC-2 still say the API never reaches it, this bullet supersedes them. The text below records the state before that commit: the API cannot originate SR Policy attributes. `send bgp ... update text ... nlri ipv4/sr-policy` reaches `parseSRPolicySection`, which carries only `distinguisher`, `color` and `endpoint` and refuses every other token as an unknown keyword, so no Binding SID, segment list, preference or label can be sent over the API, and the D2 range check is never reached from it. The ExaBGP bridge emits that text with its tunnel-encap tokens and meets the same refusal (journal row, `plan/journal/parser-refuses-a-form-its-producer-emits.md`, 2026-10-14). Wiring the API to the SR Policy attributes is a separate feature, homed since 2026-10-17 in `plan/immediate/spec-exabgp-6-api-compatibility.md` (Phase 5), which absorbed the skeleton created 2026-10-14 on the owner's answer; this spec does not change it

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
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

## Design Review Log

| Date | Round | Outcome |
|------|-------|---------|
| 2026-10-14 | design review r1 (0 BLOCKER, 9 ISSUE, 5 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-sr-policy-design-r1.md`) | Applied. I-1 AC-6 expects 3/1 (RFC 4271 Section 6.3 assigns 3/10 to the legacy NLRI field only; MP NLRI resets leave through `rfc7606SessionReset`); I-2 "Verdict per MP_UNREACH case" table, Section 5.2 exemption recorded as a journal row and Known Limitation, not fixed here, R-3 and the fixture note corrected (MP_UNREACH, not MP_REACH); I-3 Q6 consequence restated, EVPN named, (b) recommended; I-4 Q2 (a) at `typedNLRIEdit`, no synthesis change, `rfc7606_withdraw.go` and `core-design.md` out of Files; I-5 AC-7 row by row, 5-2 unconditional; I-6 `bgp-sr-policy-malformed-receive-gobgp` inject scenario with revert-reds, origination scenario no longer claimed as discriminating; I-7 `srpolicy-api-label-range-refused.ci`; I-8 ADD-PATH reachable for SAFI 73 (`parseAddPathFromTree`), rule and splitter honour it (AC-8); I-9 line numbers replaced by symbols; N-1..N-5 applied. Q1..Q6 stay open. **Round 2 owed, narrow:** the subcode decision, the MP_UNREACH verdict table, the ADD-PATH scope (shared walk, AC-8), the inject scenario's assertions and reds, and the Q2/Q6 rewording |
| 2026-10-14 | design review r2, narrow (0 BLOCKER, 2 ISSUE, 4 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-sr-policy-design-r2.md`) | Applied. I-1 a `cmd=api:` line executes nothing (`docs/architecture/testing/ci-format.md`), so the API refusal test is driven by a new fixture plugin `plugin/srpolicy-api-label-range` (`internal/test/fixture/plugin_fixture_01_srpolicy.go`, registered in `plugin_fixture_01.go`), which also sends the in-range API announce; `bgp-srpolicy-3-encode.ci` relabeled config-driven; wiring rows, AC-2, user stories 2 and 3, Goal Validation, Files and Integration Checklist corrected. I-2 Q6 (b) names the three `NLRISyntaxRuled` sites, `RetainWellFormedNLRI` becomes BGP-LS's registered retain answer, the IPv4 unicast pass-through case of `TestRetainWellFormedNLRILeavesOtherFamiliesAlone` moves to the registry lookup with its assertion kept, and the cost is stated in full. N-1 the length-rule red is the Ze-GoBGP session drop and the D4 red is B staying installed, both read from GoBGP v4.4.0 source and unverified for the v3.31.0 image; the session-up assertion depends on Q2 (a); the unreadable "no 13-octet NLRI reaches GoBGP" check replaced by the session observable. N-2 `typedNLRIEmptied` case named in Q2 (a). N-3 RFC9830-2.1-2 unchanged (encoder-tagged) in AC-7. N-4 R-7 added. Q1..Q6 stay open. **Round 3 owed, narrow:** the fixture plugin design (I-1), the Q6 (b) cost and the moved test case (I-2), and the interop red text (N-1) |
| 2026-10-14 | design review r3, narrow (1 BLOCKER, 0 ISSUE, 2 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-sr-policy-design-r3.md`) | Applied. B-1 the API never reaches `parseConfigRoute` (`parseSRPolicySection` takes only distinguisher/color/endpoint, refusing the rest as unknown keyword), so the API claim was false: the fixture plugin `plugin_fixture_01_srpolicy.go` and `srpolicy-api-label-range-refused.ci` are dropped; the range refusal is proven through config (`srpolicy-binding-sid-label-range.ci`, in-range `bgp-srpolicy-3-encode.ci`) and `ze bgp encode` (`cli-bgp-encode-srpolicy-label-range.ci`); Current Behavior constraint, Entry Point, wiring row, AC-2, user stories 2 and 3, Functional Tests, Goal Validation, Integration and Documentation checklists, Files and Key Insights corrected; Known Limitations entry added; ExaBGP bridge refusal journal row in `plan/journal/parser-refuses-a-form-its-producer-emits.md`. N-1 End-of-RIB allowed in `bgp-srpolicy-3-encode`. N-2 RFC9830-2.1-2 tagging stated in AC-7. Q1..Q6 stay open. **Round 4 owed, narrow:** the `ze bgp encode` test design and the removed API claim |
| 2026-10-14 | design review r4, narrow (0 BLOCKER, 0 ISSUE, 2 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-sr-policy-design-r4.md`) | N-1: the encode test pins each sub-TLV whole (`0D060000FFFFF000`, `01060000FFFFF000`). N-2: the journal row narrowed to announces carrying an attribute token. Design converged; Q1..Q6 and the API-attributes scope question stay with the owner |
| 2026-10-14 | owner answers | Owner: "Do what you think is right" = accept the recommendation on every open question. Recorded: Q1 (a) session reset for unwalkable framing (AC-6 unconditional); Q2 (a) malformed NLRIs dropped at `typedNLRIEdit`; Q3 (a) RFC9830-4.2.1-4/-5/-8 stay gaps, markers narrowed; Q4 (a) untagged width regression tests; Q5 (a) Type A accepts 0-15; Q6 (b) one NLRI-syntax registry for BGP-LS, EVPN and SR Policy. Unchosen branches removed from Current Behavior, Design, AC-3/6/7/8, TDD, Functional and Interop tests, Files, Documentation row 16, Implementation Steps and Key Design Decisions; R-5 and R-7 void. The API-attributes question is homed in a skeleton spec, absorbed on 2026-10-17 by `plan/immediate/spec-exabgp-6-api-compatibility.md`, cited from Known Limitations. No new design owed: every chosen branch was already designed and reviewed (r1 to r4). Status stays `design` until the main thread moves it to `ready` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-sr-policy-rfc-defects-0b4a8e87-9c06-4f85-90a2-0445cb843906.md` (verdict `clean`, rounds 3, 49 files, machine-local) |
| `./le spec review check` | OK after round 3 (0 BLOCKER, 0 ISSUE); interop revert-reds (N-3) still owed |
| Rounds | 3. Round 3 (2026-10-21, a third independent reviewer, narrow: the round 2 fixes `4e01d13a6e`, `7d6406fb5d`, `49f53c4a9f`, `cb7a3ca68f`, `ce9193df5d`, `11e72ea79e`, plus `69088b9fd5`). Round 1 (2026-10-20, independent reviewer, over commits `6ca8175fbc`, `8da63d389a`, `02b886d7b1`, `b73176e7ea`, `ca5ae1feaa`, `65ecdbf014` as at HEAD `df7d3abc23`). Round 2 (2026-10-21, a second independent reviewer, narrow: the round 1 fixes `7052b662a4`, `55be07dd57`, `11ea7bebde` and what they touch) |
| Reviewer lenses used | Round 1: wiring + functional coverage; logic + removed-behavior + test rewrites; RFC 9830 / RFC 7911 + docs drift. Round 2: disposition verification at the producer; RFC 7911 framing logic per rail and family; test discrimination of each new line. Round 3: disposition verification at the producer; removed-behavior (parser at `4e01d13a6e^` against HEAD); test discrimination by mutation in an export; docs drift |

### Findings open (round 1)
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| B-1 | BLOCKER | The receive walk now skips a 4-octet path identifier whenever ADD-PATH receive is negotiated for SAFI 73 (AC-8), but Ze's own SR Policy origination never writes one: the plugin route loop calls `GetUpdateBuilder(..., addPath)` then `BuildPlugin`, whose `buildMPReachPlugin` copies `PluginParams.NLRI` verbatim (`toPluginParams`), and `EncodeRoute` discards `addPath`. `parseAddPathFromTree` applies the default direction to every multiprotocol family, so two Ze speakers with a default `add-path` direction and sr-policy reset the session (3/1) on every SR Policy UPDATE: the receiver reads distinguisher octets as path identifier and length. Before this change the receiver ignored `addPath` for SAFI 73 and the session stayed up. RFC 7911 puts the path identifier on every NLRI of a family once negotiated, so the defect is on the send side, and this change turned it into a session reset. Fix direction (write the path identifier on send, or stop advertising ADD-PATH for SAFI 73) is the main thread's call | `reactor/peer_initial_sync.go` (plugin route loop), `message/update_build_plugin.go::buildMPReachPlugin`, `srpolicy/encode.go::EncodeRoute`, `reactor/config_capabilities.go::parseAddPathFromTree`, `srpolicy/split.go::srPolicyNLRIBounds` | open |
| I-1 | ISSUE | AC-8 (user story 6) is proven only through `processMessage` in `TestRFC9830NLRILengthReceiveVerdicts`; no `.ci` runs a daemon with ADD-PATH negotiated for sr-policy (nothing under `test/` names both). A Ze-to-Ze one would have exposed B-1 | `test/plugin/` (absent) | open |
| I-2 | ISSUE | The new `nlrisyntax` registry has no `ai/INDEX.md` keyword row; only `ai/CODE-TO-DOCS.md` maps it, though Discovery promised an INDEX route to `docs/architecture/wire/nlri.md` "NLRI Receive Validation" | `ai/INDEX.md` | open |

### Notes (round 1, non-blocking)
| # | Note |
|---|------|
| N-1 | The spec record is stale against HEAD. `99ed958ec5` fixed the RFC 7606 Section 5.2 exemption, so "Verdict per MP_UNREACH case", Known Limitations bullet 1 and the AC-3 / AC-7 (RFC9830-5-2) text no longer hold: `rfc/short/rfc9830.md` RFC9830-5-2 carries no marker and the test asserts treat-as-withdraw for the MP_UNREACH-only shape. Also `af18a709d8` set `NLRISectionCarriesAttributes`, so `update text` sr-policy now reaches `parseConfigRoute` and the D2 range checks, against Known Limitations bullet 4 and Current Behavior |
| N-2 | `srPolicyAnnounced` tests SAFI 73 and AFI 1/2 by numeric literal, not by family constants |
| N-3 | Owed evidence, not a code defect: the two revert-reds of `bgp-sr-policy-malformed-receive-gobgp` (length rule, then the D4 branch) are to be run in the QEMU guest |

### Round 1 dispositions (2026-10-21, for round 2 to verify; the gate is not clean)
| # | Disposition | Commit |
|---|-------------|--------|
| B-1 | Fixed in the shared layer, not per family. `message.PluginNLRI` frames one NLRI with its 4-octet Path Identifier (RFC 7911 Section 3); the reactor plugin-route rail (`sendPluginRoutesVia`, each member of a grouped UPDATE included) frames every route when ADD-PATH send is negotiated for its family, and the `ze bgp encode -i` encoders (sr-policy, flowspec, mup, vpls) frame with 0. The identifier is the route's configured `path-information`, 0 when none is set, as for a static route; it travels attribute block, `PluginRouteConfig.PathID`, `reactor.PluginRoute.PathID`, and `pluginRouteGroupKey` holds it. Families that shared the defect: SR Policy, FlowSpec, MUP, VPLS, MVPN, EVPN config routes, and the four encoders. Red before: `test/encode/addpath-srpolicy-pathid.ci` (timeout, no UPDATE matched), `TestSRPolicyEncodeRouteAddPathWritesPathID` (NLRI without `00000000`), `TestPluginRouteAddPathWritesPathID` (red with the framing reverted) | `7052b662a4` |
| I-1 | `test/encode/addpath-srpolicy-pathid.ci`: ADD-PATH send/receive negotiated for ipv4/sr-policy, two config routes (no path-information, then `path-information 9`), exact UPDATE hex at `le test peer`. The Ze receive side of those bytes stays proven by `TestRFC9830NLRILengthReceiveVerdicts` (AC-8); no two-Ze `.ci` was written | `7052b662a4` |
| I-2 | `ai/INDEX.md` row for `internal/core/bgp/nlri/nlrisyntax` (hand-kept file, no generator) | `55be07dd57` |
| N-1 | Verified at source; recorded in Known Limitations bullets 1 and 4, which supersede the stale table and AC text | the spec commit carrying this table |
| N-2 | `srPolicyAnnounced` compares `family.SAFISRPolicy`, `family.AFIIPv4`, `family.AFIIPv6` | `7052b662a4` |
| N-3 | Not handled here, owed to the interop agent | - |

Verified in this round: scoped tests green (`srpolicy`, `nlrisyntax`; `reactor` and `message` filtered to the RFC9830, TypedNLRIEdit, PrefixLimitShort, SRPolicy, NLRISyntax, RetainWellFormed, BGPLS and EVPN tests). `./le rfc check` shows no RFC 9830 violation (its 4 violations are RFC 4659, RFC 8654 and RFC 8955 records). RFC9830-4.2.1-2 and -4.2.1-7 are audited `enforced`. The moved assertions of `TestRetainWellFormedNLRILeavesOtherFamiliesAlone` and `TestReceivePrefixLimitShortAddPathEntry` are kept. Every `NLRISyntaxRuled` site and the `isEVPN` literal are gone (no-layering). The wire, attribute, message and feature pages match the code.

### Round 2 (2026-10-21, independent reviewer, narrow over `7052b662a4`, `55be07dd57`, `11ea7bebde`)

Dispositions verified at the producer:

| # | Verdict | Evidence |
|---|---------|----------|
| B-1 | Fixed | `reactor/peer_initial_sync.go::sendPluginRoutesVia` frames each ungrouped route with `message.PluginNLRI(params.NLRI, route.PathID, addPath)`; `sendPluginRouteGroup` frames every member before both the one-UPDATE build and the `packNLRIs` split, so the split measures framed bytes. `addPath` is `Peer.addPathFor(fam)`, read from `sendCtx` (`bgpctx.FromNegotiatedSend`), so the framing follows ADD-PATH send per family, never receive. The Section 3 quote above `PluginNLRI` is verbatim against `rfc/full/rfc7911.txt` lines 144-146, under "3. Extended NLRI Encodings". `PathID` travels `bgp_routes.go::buildConfigRouteRequest` (`parsed.PathID`), `PluginRouteConfig.PathID`, `loader_routes.go::convertPluginRoute`, `reactor.PluginRoute.PathID`; `test/encode/addpath-srpolicy-pathid.ci` pins `00000000` and `00000009` ahead of the 0x60 length octet. Config plugin routes have no withdrawal rail (`p.settings.PluginRoutes` is read only by `sendPluginRoutesVia`), so no MP_UNREACH is owed there; the API rail frames through `WireNLRI` (`SupportsAddPath` true, `PathID` 0 without one) in `nlri.WriteNLRI`. Every receiver Ze has for these families honours `addPath` (`message/rfc7606.go::validateMPNLRIField`, `flowspec/plugin.go::DecodeNLRIHex`, `vpls/vpls.go::DecodeNLRIHex`, `srpolicy/split.go`), so no Ze receive side meets a Path Identifier it does not expect |
| I-1 | Fixed | `test/encode/addpath-srpolicy-pathid.ci` reads as the disposition says |
| I-2 | Fixed | `ai/INDEX.md` line 442 routes `nlrisyntax` to `docs/architecture/wire/nlri.md` |
| N-1 | Accepted | record correction only |
| N-2 | Fixed | `reactor/session_tunnel_encap.go::srPolicyAnnounced` compares `family.SAFISRPolicy`, `family.AFIIPv4`, `family.AFIIPv6` |
| N-3 | Still owed | interop revert-reds run separately in QEMU; owed evidence, not a finding |

`bgp/config` reds: foreign to `7052b662a4`. `go test -tags ze_core,ze_distro ./internal/component/bgp/config/` fails the same 55 tests over the working tree and over a `git archive 7052b662a4^` export (failing-name lists identical by `diff`), all on schema and registry lookups (`unknown top-level keyword: ospf`/`isis`, `unknown field in policy: reject-asn`, `unknown internal plugin`, `unknown address family "ipv4/flow"`) that the PathID return of `buildConfigRouteRequest` cannot produce. `reactor`, `message`, `srpolicy` and `mup` pass with the same tags.

### Findings open (round 2)
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| I-3 | ISSUE | The SR Policy ADD-PATH refusal is now false. `srpolicy/register.go` still registers `NLRIAddPathRefusal: "Ze builds SR-Policy routes without an ADD-PATH path identifier"`, and its comment says "The SR-Policy route Ze builds carries none". Since `7052b662a4` the config rail sends a configured `path-information 9` (`addpath-srpolicy-pathid.ci`), but `update text ... path-information 9 nlri ipv4/sr-policy add ...` is still refused by `update/update_text_nlri.go::parseRegistryNLRISection` (`errFamilyRefusesAddPath`) with that reason shown to the operator. The fix removed the matching `_ = addPath` comment in `srpolicy/encode.go` but not this one (`ai/rules/stale-comments.md`, `ai/rules/user-facing-errors.md`). `srpolicy/types.go::PathID` says "SR-Policy does not use ADD-PATH" and is stale too | `srpolicy/register.go` (`NLRIAddPathRefusal`), `srpolicy/types.go` (`PathID`, `SupportsAddPath` docs) | open |
| I-4 | ISSUE | The `ze bgp encode -i` framing in `flowspec/encode.go::EncodeRoute`, `mup/encode.go::EncodeRoute` and `vpls/encode.go::EncodeRoute` has no test: every `EncodeRoute` call in those packages' tests passes `addPath` false. Deleting any of the three `message.PluginNLRI` lines leaves every test green. Only `srpolicy` has `TestSRPolicyEncodeRouteAddPathWritesPathID` | `flowspec`, `mup`, `vpls` `*_test.go` (absent) | open |
| I-5 | ISSUE | `pluginRouteGroupKey`'s new `PathID` field is unproven. `TestPluginRouteAddPathWritesPathID` groups two MVPN routes that both have PathID 0, so it stays green if `b.Uint(uint64(r.PathID))` is removed from the key. Two grouped routes with `path-information 1` and `2` would then share one UPDATE and both go out with the representative's identifier (`sendPluginRouteGroup` uses `g.rep.PathID`) | `reactor/plugin_route_addpath_test.go` | open |

### Notes (round 2, non-blocking)
| # | Note |
|---|------|
| N-4 | `7052b662a4` also carries an unrelated RFC 4760 Section 3 change that its body does not name: `mup/encode.go::EncodeRoute` stops writing NEXT_HOP (code 3) for IPv4 MUP. Its test `TestEncodeRouteWritesNoNextHop` is still uncommitted in `mup/encode_test.go` (another session's hunk), and `internal/core/family/family.go` carries the matching uncommitted removal of `LegacyNextHop`. The change is RFC-correct and `mup` passes over the working tree; `ai/rules/git-safety.md` asks the body to say whose hunks rode along |
| N-5 | The `PathID` field in the group key also splits groups on sessions without ADD-PATH, where the identifier is never sent: routes that differ only in `path-information` now go in separate UPDATEs. Correct on the wire, just more UPDATEs |

Round 2 verdict: 0 BLOCKER, 3 ISSUE (I-3, I-4, I-5), 2 NOTE. The gate is not clean. Round 3 is owed, narrow: the I-3 to I-5 fixes only.

### Round 2 dispositions (2026-10-21, for round 3 to verify; the gate is not clean)
| # | Disposition | Commit |
|---|-------------|--------|
| I-3 | Made to work, not reworded. The refusal was the only user of `Registration.NLRIAddPathRefusal`, so the field, `registry.NLRIAddPathRefusalFor`, `errFamilyRefusesAddPath` and `refuseAddPath` are deleted. `update/update_text_nlri.go::parseRegistryNLRISection` now hands a top-level `path-information` to the sr-policy section like any registry family, and `parseRegistryRouteSection` frames the NLRI `parseConfigRoute` built through `wireNLRI(fam, route.NLRI, accum.PathID, accum.HasPathID)` (RFC 7911 Section 3). Written right after the family it fails as a missing `add`/`del`, inside the run the plugin grammar refuses it, as for mup and mvpn. `srpolicy/types.go` `PathID`/`HasPathID`/`SupportsAddPath` say the sender frames the identifier; `update-syntax.md`, `wire/nlri.md`, `plugin-system.md` and `ai/patterns/registration.md` updated. Test through the entry point: `test/plugin/update-text-sr-policy-addpath.ci` (ADD-PATH send/receive for ipv4/sr-policy, `update text path-information 7 ... mp ipv4/sr-policy add|del`, exact MP_REACH and MP_UNREACH hex with `00000007` ahead of `60`). Red before: the announce refused, "path-information not supported for ipv4/sr-policy: Ze builds SR-Policy routes without an ADD-PATH path identifier"; green after. `TestSRPolicyTextMatchesPluginGrammar` path-id rows assert the framed bytes (untagged test) | `4e01d13a6e` |
| I-4 | `TestFlowSpecEncodeRouteAddPathWritesPathID`, `TestMUPEncodeRouteAddPathWritesPathID`, `TestVPLSEncodeRouteAddPathWritesPathID`, one new `encode_addpath_test.go` per package: `EncodeRoute` with `addPath` false then true, asserting in the returned NLRI and the MP_REACH_NLRI value the `00000000` prefix, and its absence without ADD-PATH. Red with each `message.PluginNLRI` line deleted (flowspec `0301080a`, mup `0100...`, vpls `0011...` without the prefix); restored, green. `mup/encode_test.go` untouched | `7d6406fb5d` |
| I-5 | `TestPluginRouteGroupSplitsOnPathID`: two grouped ipv4/mvpn routes, equal attributes, PathID 1 and 2, ADD-PATH send: two UPDATEs, `00000001`+A and `00000002`+B. Red with `b.Uint(uint64(r.PathID))` removed from `pluginRouteGroupKey`: one UPDATE, both NLRIs behind `00000001` | `49f53c4a9f` |
| re-record | `rfc/discrimination/rfc2545.json`: RFC2545-3-1 positive and RFC2545-3-2 positive (`TestSRPolicyEncodeNextHopWire`), RFC2545-3-1 negative (`TestSRPolicyEncodeRejectsInvalidIPv6NextHop`), producer `srpolicy/encode.go::EncodeRoute`, re-recorded by `./le rfc discriminate-record` (green clean, red halted); `./le rfc check` no longer reports them | `cb7a3ca68f` |

Owed to round 3 and the main thread: `./le verify worktree` over these commits (verification debt `plan/verification-debt/9ce43bcc.md`). `./le rfc check` still reports producer-changed records for RFC9830-2.1-3 at `reactor_api_batch.go::planBatchAttrs`, BFD strict, RFC 2918, RFC 7705 and RFC 7950; none is produced by these commits.

### Round 3 (2026-10-21, independent reviewer, narrow over `4e01d13a6e`, `7d6406fb5d`, `49f53c4a9f`, `cb7a3ca68f`, `ce9193df5d`, `11e72ea79e`, and the test fix `69088b9fd5`)

Dispositions verified at the producer:

| # | Verdict | Evidence |
|---|---------|----------|
| I-3 | Fixed | `update/update_text_nlri.go::parseRegistryNLRISection` no longer consults a refusal; an `add` run reaches `parseRegistryRouteSection`, which frames `route.NLRI` through `wireNLRI(fam, route.NLRI, accum.PathID, accum.HasPathID)`; a `del` run goes through `encodeViaRegistry` with the same identifier. No `NLRIAddPathRefusal`, `NLRIAddPathRefusalFor`, `errFamilyRefusesAddPath` or `refuseAddPath` is left in the tree outside `plan/` (one historical mention in the `.ci` PREVENTS comment). `srpolicy/types.go` `PathID`/`HasPathID`/`SupportsAddPath` comments match the code. `test/plugin/update-text-sr-policy-addpath.ci` hex checked by hand: MP_REACH `800E1A` (26 = 2+1+1+4+1+4+1+12) carries `00000007 60 00000000 0000012C 0A000003`; MP_UNREACH `800F14` (20); UPDATE lengths `0x79` and `0x3C` add up. Behaviour change judged: a `path-information` right after the family, before `add`/`del`, was never accepted for a registry family. At `4e01d13a6e^` mup and mvpn (no refusal registered) already answered `ErrMissingAddDel` from the `mode == ""` branch, and sr-policy answered `errFamilyRefusesAddPath`, so no previously accepted form is lost; native families never reach this parser. `update-syntax.md` states the registry-family exception beside the general `mp <family> [<modifier>]*` grammar |
| I-4 | Fixed | `flowspec`, `mup`, `vpls` `encode_addpath_test.go` call `EncodeRoute` with `addPath` false then true and compare the returned NLRI and the MP_REACH NLRI field against `00000000` + the bare NLRI. Each `message.PluginNLRI(nlriBytes, 0, addPath)` line replaced by `_ = addPath` in a `git archive HEAD` export: all three red (`0301080a`, `0100...`, `0011...` without the prefix). A framing that ignored `addPath` and always framed would also go red (bare NLRI then carries the prefix twice in `want`) |
| I-5 | Fixed | `TestPluginRouteGroupSplitsOnPathID` red in the export with `b.Uint(uint64(r.PathID))` removed from `reactor/peer_initial_sync.go::pluginRouteGroupKey`; UPDATE order is deterministic (`slices.Sort(groupOrder)` in `sendPluginRoutesVia`) |
| re-record | Accepted | `cb7a3ca68f` moves only `producer-sha` for the three RFC 2545 records (same units, same break); `./le rfc check` at HEAD reports no RFC 2545 record for `srpolicy/encode.go::EncodeRoute` |
| docs | Accepted | `ce9193df5d` test anchors resolve (`TestPluginRouteGroupSplitsOnPathID`, the `.ci`) |
| test fix `69088b9fd5` | Correct | draft-simpson-idr-flowspec-redirect-02 Section 3 (read from the IETF archive text; not under `rfc/drafts/`): routers "redirect them (C=0) or copies them (C=1) towards the IPv4 or IPv6 address specified in the 'Network Address of Next-Hop' field of the associated MP_REACH_NLRI", and the community is ignored when "the length of the next-hop is 0". The new hex carries `800E10 0001 85 04 01020304 00` and extended community `0800 000000000001` (C bit set, `copy-to-nexthop`); lengths `0x43`, `0x2C`, `0x10` add up. `4358372547` (`message.MPReachCarriesNextHop`) is the producer |

Mutation run: one `git archive HEAD` export under the session scratch, five lines mutated there (never the shared tree); `go test -tags ze_core,ze_distro` red in flowspec, mup, vpls, reactor and update (`path-id/top_level_add`, `path-id/top_level_mixed`). Unmutated HEAD: the same tests green.

RFC9830-2.1-3 producer-changed record (`reactor_api_batch.go::planBatchAttrs`, recorded `a4807de00d`): no commit of this spec changed it. `git log -L` over `planBatchAttrs` since `a4807de00d` names only `e7354f3351` (body: the `legacyNextHopApplies` arm deleted, RFC 4760 Section 3) and `035b795734` (comment edit), both carrying verification debt `485edca9`, the RFC 4760 NEXT_HOP work of another session. `e7354f3351` sits inside `7052b662a4..HEAD` by date only. The re-record belongs to that work.

### Notes (round 3, non-blocking)
| # | Note |
|---|------|
| N-6 | `docs/architecture/wire/nlri.md` row "a `path-information` inside the run: refused by the config-route parser" holds for an `add` run; a `del` run goes to `srpolicy/encode.go::EncodeNLRIHex`, whose `parseSRPolicyNLRIArgs` default arm refuses it. It is refused either way; no unit row drives `path-information` inside a `del` run |
| N-7 | `test/plugin/update-text-sr-policy-addpath.ci` MP_REACH comment writes the NLRI as `60 0000000000000000012C 0A000003` (14 octets); the expect hex is right |
| N-3 | Still owed: the interop revert-reds of `bgp-sr-policy-malformed-receive-gobgp`, run separately. Owed evidence, not a finding |

Round 3 verdict: 0 BLOCKER, 0 ISSUE, 3 NOTE. The code review is clean. The artifact is re-recorded `clean`, rounds 3, 49 code files of this spec's commits, `./le spec review check` OK; `internal/component/bgp/plugins/nlri/mup/encode.go` is left out of the pin because it holds another session's uncommitted hunk (it does not touch the framing line). Still owed before closure: the interop revert-reds (N-3) and `./le verify worktree` (verification debt `9ce43bcc`).

# Spec: bgp-mup-segment-discovery-originator-checks -- MUP DSD and ISD routes checked against the locator originator

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Four MUST rows of the implemented MUP SAFI are unmet: a Segment Discovery route that should be withdrawn is stored. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split out of spec-bgp-unowned-rfc-rows on 2026-11-14 (owner: "split
bgp-unowned-rfc-rows"). That spec closes on its proven rows; this one receives
every MUP row it held, with the acceptance criteria, tests and design text
unchanged. Row numbers are the numbers the rows had there, kept so the quotes
and comparisons stay traceable.

| # | Row id | Section | State on 2026-11-14 | What Ze does today |
|---|--------|---------|---------------------|--------------------|
| 1 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4` | draft-ietf-bess-mup-safi §3.3.6 | `{gap}`, no code | Stores a received Direct Segment Discovery (DSD) route without comparing its MP_REACH next hop to the originator of the Prefix-SID locator |
| 2 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5` | same | implemented, tagged in both polarities and recorded; two proofs owed (below) | Withdraws a DSD route that arrives with no Prefix-SID (`mup.CheckDSDPrefixSID`, registered through `nlritype.RegisterUpdateCheck`) |
| 13 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1` | draft-ietf-bess-mup-safi §3.3.3 | `{gap}`, no code | Stores a received Interwork Segment Discovery (ISD) route without checking that its Address field is an address of the originator of the Prefix-SID locator |
| 14 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3` | same | `{gap}`, no code | Stores a received ISD route that carries no Prefix-SID attribute |
| 15 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4` | same | `{gap}`, no code | Never detects a mismatch between an ISD route's next hop and the Prefix-SID locator originator |

Row 2 moved with its owed proofs, because a wire-visible change owes an interop
test (`ai/rules/interop-and-goal-validation.md`) and none exists for it:

| Owed for row 2 | Why it is owed |
|----------------|----------------|
| Interop scenario `bgp-mup-dsd-gobgp` | No scenario under `test/interop/scenarios/` sends a DSD route from another implementation. `bgp-mup-malformed-t1st-withdraw-gobgp` covers Type 1 ST routes only |
| `test/plugin/mup-dsd-ebgp-prefix-sid-discarded-withdraw.ci` | AC-2's RFC 8669 §4 EBGP discard arm is proven by a unit case only (`TestMUPDSDWithoutPrefixSIDWithdrawnOnReceive`, case "Prefix-SID discarded from an EBGP peer"); the daemon path has no test. `test/plugin/mup-dsd-without-prefix-sid-withdraw.ci` runs with `accept-srv6-prefix-sid true` |

What landed for row 2, to audit before implementing (`/ze-audit`), not to
rebuild: `internal/core/bgp/nlri/nlritype/update_check.go` (`UpdateCheck`,
`UpdateAttrs`, `RegisterUpdateCheck`, `Rulings`),
`internal/component/bgp/plugins/nlri/mup/register.go` (registers
`CheckDSDPrefixSID`, defined in `rfc7606.go`),
`internal/component/bgp/reactor/session_validation_nlritype.go`
(`applyTypedNLRIDiscard` returns withdraw-only bodies placed by
`typedNLRIPlaceWithdrawals`),
`internal/component/bgp/reactor/draft_ietf_bess_mup_safi_dsd_prefix_sid_test.go`
(`TestMUPDSDWithoutPrefixSIDWithdrawnOnReceive`,
`TestMUPDSDWithPrefixSIDKeptOnReceive`),
`rfc/discrimination/draft-ietf-bess-mup-safi/DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5.json`
and `test/plugin/mup-dsd-without-prefix-sid-withdraw.ci`.

### Design approval

The owner approved the design of rows 1 and 2 on 2026-10-11 ("Approve", after
the round-5 design review, 0 BLOCKER) inside spec-bgp-unowned-rfc-rows. Rows 13
to 15 joined on 2026-10-21 and reuse that mechanism; the Existing Mechanisms
section below is new. Status is `design` until the main session has presented
the whole to the owner.

### The RFC text, quoted from the source

| # | Quote | Read at |
|---|-------|---------|
| 1 | "However, the receiving BGP speaker MUST ensure that the received nexthop value in the MP_REACH_NLRI attribute is identical to the originator of the locator value in the prefix SID attribute. The originator of the locator value can be resolved from the IPv6 IGP table. If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied." | `rfc/drafts/draft-ietf-bess-mup-safi.txt` §3.3.6 |
| 2 | "When a BGP speaker receives a MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied." | same, §3.3.6 |
| 1, 2 (skip) | "A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message." (follows both DSD sentences) | same, §3.3.6 |
| 13 | "However, the receiving BGP speaker MUST ensure that the value of Address filed in the NLRI is an address of the originator of the locator value in the prefix SID attribute." | the row in `rfc/short/draft-ietf-bess-mup-safi.md`, a verbatim span of §3.3.3; re-read in `rfc/drafts/draft-ietf-bess-mup-safi.txt` at design |
| 14 | "When a BGP speaker receives the Interwork Segment Dicovery routes with a MP_REACH_NLRI attribute without a prefix SID attribute, then it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied." | same |
| 15 | "If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied." | same |

### How FRR and BIRD handle it

Neither implements the MUP SAFI: no MUP code in FRR `bgpd/` nor in BIRD
`proto/bgp/`. GoBGP does, from v3.6.0 (A-4).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/plugin-system.md` - "The process boundary"
  → Constraint: a plain exported Go function called across packages reaches process-local state and "does nothing useful when the plugin runs external ... There is no error, no panic and no log line". The `locatororigin` resolver is that class, so it is guarded by `.IsInternal()` and listed in `dangerousCalls` (Design resolutions, R2-I1)
- [ ] `pkg/plugin/rpc/bridge.go` - `DirectBridge`, the in-process transport of an internal plugin
  → Decision: not used for the originator lookup: it serves only an internal plugin, so it adds nothing the guarded registry lacks, and a request and response per DSD NLRI would put an RPC on the receive path (R2-I1)
- [ ] `docs/architecture/testing/interop.md` - scenario naming and the revert-and-red walk
  → Constraint: the scenario is named (no numeric prefix) and owes a recorded red with the fix reverted and the artifact rebuilt

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/draft-ietf-bess-mup-safi.md` - rows 3.3.3-1, 3.3.3-3, 3.3.3-4, 3.3.6-4, 3.3.6-5
  → Constraint: the `{gap}` annotations name `RecognizeNLRI` (`internal/component/bgp/plugins/nlri/mup/rfc7606.go`) and `DecodeNLRIHex` (`mup.go`) as reading the architecture and route type alone
- [ ] `rfc/full/rfc9352.txt` §7.1 (lines 453-462) and `rfc/full/rfc7794.txt` §2.1 (X-Flag lines 210-214, R-Flag lines 231-233) and §2.2 (IPv6 Source Router ID, type 12, lines 247-278)
  → Constraint: "The Prefix Attribute Flags sub-TLV MUST be included in the Locator TLV when it is leaked upwards in the hierarchy or originated as a result of the redistribution from another protocol or another IS-IS instance." (RFC 9352 §7.1). Only flags present with R and X clear prove an originator; a Source Router ID equal to the next hop is a match (Design resolutions, B1)
- [ ] `rfc/full/rfc9352.txt` §6 (lines 324-372), `rfc/full/rfc7794.txt` §2.1 X-Flag in TLVs 236 and 237 (lines 218-221), `rfc/full/rfc5308.txt` §2 external bit (lines 107-108)
  → Constraint: "If at least one of them sets the A-flag in its advertisement, the prefix/SRv6 Locator SHOULD be considered as anycast." and "In case of any inconsistency between the Prefix Attribute Flags advertised in the Locator TLV and in the Prefix Reachability TLV, the ones advertised in the Prefix Reachability TLV MUST be preferred." (RFC 9352 §6). The A-flag (bit 4) on any covering advertisement is the anycast not-known; a same-system TLV 236 entry for the prefix decides X by its external bit whether or not it carries a Prefix Attribute Flags sub-TLV, because the bit is in TLV 236's fixed header and RFC 9352 §6 says "When included in the Locator TLV, the state of the X-flag in the Prefix Attributes Flags sub-TLV MUST match the setting of the embedded "X-bit" in any advertisement for the same prefix in TLVs 236 [RFC5308] and 237 [RFC5120]." (lines 365-368), while RFC 7794 §2.1 says of the sub-TLV X-Flag "In TLVs 236 and 237, this flag SHOULD always be sent as 0 and MUST be ignored on receipt"; that sub-TLV, when present, decides R and A (Design resolutions, B1, round-4 I1, round-5 I1)

**Key insights:**
- Row 2's mechanism exists: a per-family UPDATE-level check beside the type recognizer. Rows 1, 13, 14 and 15 are further rulings on it, never a second mechanism.
- Rows 1, 13 and 15 need the locator's originator, which only the IS-IS LSDB holds.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/nlri/mup/rfc7606.go` - `RecognizeNLRI` and `MUPRouteType.Implemented` decide per-NLRI recognition from architecture and route type only; DSD and ISD are implemented types. `CheckDSDPrefixSID` keeps every route that is not a DSD route, and withdraws a DSD route when `attrs.Has(attribute.AttrPrefixSID)` is false
- [ ] `internal/core/bgp/nlri/nlritype/update_check.go` - `UpdateCheck` takes one carved NLRI, the ADD-PATH flag and an `UpdateAttrs` view; `UpdateAttrs.Has` answers attribute presence only, so a check cannot read the Prefix-SID's SID or the MP next hop today
- [ ] `internal/component/bgp/plugins/nlri/mup/register.go` - registers `CheckDSDPrefixSID` per MUP family through `nlritype.RegisterUpdateCheck`
- [ ] `internal/component/bgp/reactor/session_validation_nlritype.go` - `applyTypedNLRIDiscard` rewrites the MP attributes (`rewriteMPNLRISections`) and returns withdraw-only bodies for the routes the family's rulings withdraw (`typedNLRIPlaceWithdrawals`)
- [ ] `internal/component/bgp/reactor/session_validation_receive.go` - an EBGP Prefix-SID is discarded under RFC 8669 §4 unless the session accepts it, before the typed-NLRI pass
- [ ] `internal/component/bgp/message/rfc7606_withdraw.go` - `SynthesizeWithdrawFamilies`, `buildWithdrawBody`, `appendMPUnreach`
- [ ] `internal/plugins/isis/bgpls_export.go` - the native LSDB export; `prefixAttributes` takes the IPv6 X from the TLV 236 header and clears the sub-TLV's (to re-read at design)
- [ ] `internal/plugins/isis/lsdb/lsdb.go` - one LSDB per level ("held separately for Level 1 and Level 2"), one RWMutex
- [ ] `internal/plugins/isis/register.go` - `runISISEngine` builds the SDK plugin from the connection; `reg.CLIHandler` runs `cli.RunPlugin`, so IS-IS can run as an external process
- [ ] `internal/le/plugin/boundary/pluginboundary.go` - `dangerousCalls` is the watch list; `.IsInternal()` and `warnIfExternal(` are the guard patterns
- [ ] `internal/le/interoplab/bgp/check_special.go` - `specialCheckers` binds a scenario name to its checker

**Behavior to preserve:**
- A DSD route with a Prefix-SID is stored; a DSD route without one is withdrawn and the rest of the UPDATE is processed (row 2, landed).
- A family with no registered check is unaffected, and the engine names no family.

**Behavior to change:** rows 1, 13, 14 and 15, as the ACs state.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-family UPDATE-level NLRI check | `internal/core/bgp/nlri/nlritype/update_check.go`, `UpdateCheck`, `RegisterUpdateCheck`, `Rulings` | Lets a family's plugin judge one carved NLRI against the UPDATE's attributes; MUP registers `CheckDSDPrefixSID` for row 2 | Extends: rows 14, 1, 13 and 15 are further rulings of the MUP check, and `UpdateAttrs` gains what they read (the Prefix-SID's SID and the MP next hop). No second registry |
| Per-NLRI type recognizer | `internal/core/bgp/nlri/nlritype/nlritype.go`, `Recognizer`, `Register`, `RegisterBodyRecognizer` | Answers about one NLRI alone, for the RFC 7606 §5.4 discard | Does not apply: it never sees the path attributes (A-1) |
| Withdraw of rejected NLRIs on receive | `internal/component/bgp/reactor/session_validation_nlritype.go`, `applyTypedNLRIDiscard` and `typedNLRIPlaceWithdrawals` | Removes rejected NLRIs from MP_REACH and withdraws them, in the UPDATE or in withdraw-only bodies the caller dispatches, as row 2 does today | Extends: the new rulings return the same `nlritype.Withdraw` verdict. The I1 text below describes the design as approved, before this path landed; the implementer audits which of it exists before adding anything |
| Core-leaf registry filled from a plugin's `init()` | `internal/core/ipsecinventory/registry.go`, `Register`; `ai/doors/core.md`, "Copy the core-leaf registry shape" | A registry in `internal/core/` that a plugin fills and a reader queries, answering not-registered rather than an empty value | Extends the shape: `internal/core/locatororigin` is a new instance of it, so BGP never imports IS-IS |
| Process-boundary watch list | `internal/le/plugin/boundary/pluginboundary.go`, `dangerousCalls` | Refuses an in-process call that silently does nothing when the plugin runs forked | Extends: `locatororigin.Register` joins the list (R2-I1) |
| IS-IS X-flag precedence | `internal/plugins/isis/bgpls_export.go`, `prefixAttributes` | Takes the IPv6 X from the TLV 236 header and clears the sub-TLV's | Extends by reuse: the resolver calls or mirrors it with a comment naming it, so the tree holds one statement of the rule (round-5 N1) |
| RFC 8669 §4 EBGP Prefix-SID discard | `internal/component/bgp/reactor/session_validation_receive.go` | Discards a Prefix-SID from an EBGP peer not configured to accept it; `UpdateAttrs.Discarding` makes it absent to a check | Does not change it: the checks see the UPDATE as it stands after the discard, which AC-2 and AC-9 state |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Receive: UPDATE wire bytes on a peer session carrying MUP NLRIs (AFI 1 or 2, SAFI 85).

### Transformation Path
1. Wire parse and RFC 7606 classification in `internal/component/bgp/message/`.
2. The RFC 8669 §4 EBGP Prefix-SID discard (`session_validation_receive.go`).
3. The typed-NLRI pass (`session_validation_nlritype.go`): per NLRI, the family's recognizer and its registered UPDATE check; the MUP check asks `locatororigin.Resolve` for rows 1, 13 and 15.
4. Kept NLRIs go on to the RIB plugin; rejected NLRIs are withdrawn.

### Design resolutions from the independent review (2026-10-11), carried unchanged

| Finding | Design |
|---------|--------|
| B1, row 1 originator (revised by round-2 I2) | The originator of a locator exists only in the IS-IS LSDB. A new registry in `internal/core/` (`locatororigin`: `Register(name, Resolver)`, `Resolve(sid netip.Addr)` answering the originator's address set, whether it is known with the reason when it is not, and an error) is filled by the IS-IS plugin at `init()`. The DSD Prefix-SID carries a SID, which "consists of the PE locator followed by a function" (draft §3.3.4), not a locator prefix, so `Resolve` takes the SID address and longest-matches it against the SRv6 Locator TLVs (27) the LSDB holds; the locator length comes from the advertised TLV 27, never from the BGP side. The originator is a router and a router has many IPv6 addresses, so the answer is the SET the single originating LSP advertises: TLV 140 (IPv6 TE Router ID), every TLV 232 (IPv6 Interface Address) and every host /128 in TLV 236. Which address BGP compares follows the MP_REACH Next Hop length (round-3 I3): 16 octets, that address; 32 octets, the global first half (the Link-Local second half is never an address the LSP advertises); 4 octets, an IPv4 address, which is in no originator set because the set holds IPv6 addresses only and the draft requires one ("a PE MUST use the IPv6 address of the PE as the nexthop address in the MP_REACH_NLRI attribute", §3.3.4), so against a known originator it is a proven mismatch and against a not-known one the route is kept like any other; any other length is not-known, "next hop length N not comparable". A member of the set is stored, and only a next hop in none of them, against a known originator, is a proven mismatch and withdraws. Which system is the originator (round-3 B1): Ze holds one LSDB per level (`internal/plugins/isis/lsdb/lsdb.go`, "held separately for Level 1 and Level 2"), and in a multi-level domain the Level 2 copy of a Level 1 PE's locator is the leaking L1/L2 router's, whose addresses do not hold the PE's next hop. So the index spans both levels and keys advertisers by system ID: every fragment and both levels of one router are one originator, and its address set is the union of those LSPs' TLV 140, TLV 232 and host /128 TLV 236 addresses. The resolver walks the sub-TLVs of each covering TLV 27 (new code: `bgpls_export.go` stores TLV 27 raw today). An IPv6 Source Router ID sub-TLV (RFC 7794 §2.2, type 12) on a covering advertisement whose address equals the next hop is a known match, stored. Advertisements whose Prefix Attribute Flags sub-TLV (RFC 7794 §2.1, type 4) has R ("Set when the prefix has been leaked from one level to another (upwards or downwards)") or X ("Set if the prefix has been redistributed from another protocol") set are not the originator. An advertisement with no Prefix Attribute Flags sub-TLV proves no originator either: RFC 9352 §7.1 makes the sub-TLV a MUST only on a leaked or redistributed locator and says that without it "receivers will be unable to determine the correct source of the advertisement", so a receiver cannot tell it from a leaked one. Anycast and flag precedence (round-4 I1): RFC 9352 §6: "The same prefix/SRv6 Locator can be advertised by multiple routers. If at least one of them sets the A-flag in its advertisement, the prefix/SRv6 Locator SHOULD be considered as anycast." So the A-flag (bit 4) set on any covering advertisement, at either level and from any system, leaked or not, is the anycast not-known whatever the R/X-clear count, because two PEs sharing an anycast locator in two Level 1 areas leave an L1L2 Ze one R-clear system and only R-set copies of the other ("A-flag MUST be preserved when the advertisement is leaked between levels", same section). RFC 9352 §6 also sets precedence: "In case of any inconsistency between the Prefix Attribute Flags advertised in the Locator TLV and in the Prefix Reachability TLV, the ones advertised in the Prefix Reachability TLV MUST be preferred." So where the same system advertises the locator prefix in a TLV 236 entry, that entry's own external bit decides X whether or not the entry carries a Prefix Attribute Flags sub-TLV (round-5 I1): the bit sits in TLV 236's fixed header, so every entry carries it (`packet.DecodeIPv6ReachabilityTLV` decodes it as `External`), RFC 5308 §2 sets it ("If the prefix was distributed into IS-IS from another routing protocol, the external bit SHALL be set to 1"), and RFC 9352 §6 binds the locator's X to it: "When included in the Locator TLV, the state of the X-flag in the Prefix Attributes Flags sub-TLV MUST match the setting of the embedded "X-bit" in any advertisement for the same prefix in TLVs 236 [RFC5308] and 237 [RFC5120]." (`rfc/full/rfc9352.txt` lines 365-368). The sub-TLV's own X-Flag is never read in TLV 236, because RFC 7794 §2.1 says of it "In TLVs 236 and 237, this flag SHOULD always be sent as 0 and MUST be ignored on receipt". When the TLV 236 entry carries a Prefix Attribute Flags sub-TLV (`packet.decodeSubTLVs` keeps TLV 236 sub-TLVs as opaque spans today), that sub-TLV decides R and A; with no sub-TLV, R and A stay from TLV 27. Ze already applies this X rule once, in BGP-LS export: `prefixAttributes` in `internal/plugins/isis/bgpls_export.go` takes the IPv6 X from the TLV 236 header and clears the sub-TLV's (its comment cites RFC 9085 §2.3.2). The resolver reuses that rule, or mirrors it with a comment naming `prefixAttributes`, so the tree holds one statement of it rather than two that can drift (round-5 N1). TLV 237 is decoded nowhere, which is already a not-known below. Exactly one system whose covering locator carries the flags (after that precedence) with R and X clear, and no covering advertisement with A set, is the originator: RFC 9352 §6, "A prefix/SRv6 Locator that is advertised by a single node and without an A-flag MUST be considered node specific." None is not-known, "locator re-advertised by <system-id>; originator not visible in this LSDB", or "locator from <system-id> carries no Prefix Attribute Flags sub-TLV; RFC 9352 §7.1: its source cannot be determined"; two or more such systems, or the A-flag on any covering advertisement, is the anycast not-known. Not known, each a named reason the keep-and-Warn arm (R-3) prints, never a zero address or an empty set read as a mismatch: no resolver registered; IS-IS engine not running in the daemon process (not configured, stopped, or run external, R2-I1); no advertised locator covers the SID; two or more systems advertise the longest covering locator with Prefix Attribute Flags present and R and X clear (anycast); any covering advertisement sets the A-flag (anycast, RFC 9352 §6); the covering locator is re-advertised (R or X set) with no Source Router ID equal to the next hop; the covering locator carries no Prefix Attribute Flags sub-TLV; a Next Hop length other than 4, 16 or 32 octets; or the next hop is in none of the decoded TLVs while the LSP carries a TLV 237 (MT IPv6 reachability), which Ze's IS-IS decodes nowhere (`internal/plugins/isis/packet/tlv.go` names 232 and 236 and no 237; grep 2026-10-11), so its host routes cannot rule the next hop out |
| I1, A-1 edit path (revised by round-2 I3 and round-3 I1) | `rewriteMPNLRISections` in `session_validation_nlritype.go` removes the rejected DSD NLRIs from MP_REACH, as it removes the §5.4 discards today, and changes nothing else, so the kept UPDATE never gains an attribute and never holds a second MP_UNREACH (RFC 7606 §3(g): "If the MP_REACH_NLRI attribute or the MP_UNREACH_NLRI [RFC4760] attribute appears more than once in the UPDATE message, then a NOTIFICATION message MUST be sent"; §5.1 says an implementation "MUST still be prepared to receive these fields in any position or combination", so another family's MP_UNREACH may already be there, and it stays byte-identical). The rejected NLRIs leave, in every case, as their own withdraw-only UPDATE body: no Withdrawn Routes, no NLRI, and one MP_UNREACH_NLRI of the MUP AFI/SAFI. A new exported `message.SynthesizeWithdrawNLRI(afi, safi, nlri)` in `rfc7606_withdraw.go`, beside `SynthesizeWithdrawFamilies`, builds it as `buildWithdrawBody(nil, nil, appendMPUnreach(...))`, the two helpers `SynthesizeWithdrawFamilies` already uses, so the Extended Length rule stays declared once, in `message.appendMPUnreach`, and the reactor declares no function of that name. `applyTypedNLRIDiscard` returns the body beside the rewritten UPDATE, `applyRFC7606` passes it out, and `session_read.go` dispatches it through the loop that already carries `SynthesizeWithdrawFamilies`' further bodies (`wireu.NewWireUpdate` with the source context and source ID, `onMessageReceived` with the non-pool `BufHandle{ID: noPoolBufID}`, so the forward cache holds it), on the normal arm as well as the treat-as-withdraw arm. When the rewrite removed every route of the UPDATE (`typedNLRIEmptied`), the withdraw body is still dispatched: the "nothing left to withdraw" return in `session_read.go` drops the emptied UPDATE, never the withdraw body. Each dispatched body holds at most one MP_UNREACH, the RIB's first-match read sees it, and this edit creates no UPDATE that holds MP_REACH beside MP_UNREACH. Audit note, 2026-11-14: row 2 landed after this text was written. No `SynthesizeWithdrawNLRI` exists in `internal/component/bgp/message/`; `applyTypedNLRIDiscard` already returns withdraw bodies through `typedNLRIPlaceWithdrawals`, which joins the withdrawal to the UPDATE while it carries no MP_UNREACH_NLRI and the result fits, and otherwise uses withdraw-only bodies. The implementer extends that path and builds from this text only what is missing |
| R2-I1, process boundary | `locatororigin.Resolve` reaches IS-IS package state through a plain exported call, the class `docs/architecture/plugin/plugin-system.md` "The process boundary" names; IS-IS can run external (`reg.CLIHandler` runs `cli.RunPlugin`, `internal/plugins/isis/register.go`). Mechanism: the registry guarded by `.IsInternal()`, not a DirectBridge request and response. A DirectBridge serves only an internal plugin (`pkg/plugin/rpc/bridge.go`), so it gives an external IS-IS nothing the guarded registry does not, and a request and response per DSD NLRI is an RPC on the receive path, which the Security checklist forbids. The resolver reads an engine handle that `runISISEngine` publishes only when `p.IsInternal()` (`sdk.Plugin.IsInternal` in `pkg/plugin/sdk/sdk.go`) and clears when the engine stops; the resolver is registered at `init()` whether or not IS-IS is configured, so with no published handle it answers not-known "IS-IS engine not running in the daemon process". When IS-IS runs external, `runISISEngine` calls a package `warnIfExternal` (the `internal/plugins/static/register.go` shape: warn, never refuse, because IS-IS routing still works external) that logs once at start: "isis: running as an external plugin process, so the BGP MUP Direct Segment Discovery next-hop check cannot read this LSDB and keeps every DSD route unverified; run isis internal to enable the check". The daemon's per-route keep-Warn for the not-running state names both causes and both fixes (configure IS-IS, or run it internal), because the daemon process cannot tell "not configured" from "external" without asking the plugin manager, which this spec does not couple to. `locatororigin.Register` joins `dangerousCalls` in `internal/le/plugin/boundary/pluginboundary.go`, so `./le plugin boundary check` refuses a future registrant whose package carries no guard. The resolver takes the LSDB read lock (`LSDB.mu`, `internal/plugins/isis/lsdb/lsdb.go`) and reads a locator-to-LSP index the engine rebuilds on LSP install, purge and expiry, never a walk per NLRI (review N2) |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to MUP plugin | the registered `nlritype.UpdateCheck`, with the attribute view the check reads | No |
| BGP (MUP check) to IS-IS (locator originator) | `internal/core/locatororigin` registry: IS-IS registers a resolver at `init()`, the MUP check calls `Resolve`; neither imports the other. Same process only: the resolver answers from an engine handle published under `.IsInternal()`, and not-known otherwise (R2-I1) | No |
| Engine to RIB plugin | received UPDATE with the per-NLRI withdraw already decided | No |

### Integration Points
- `nlritype.RegisterUpdateCheck` and `UpdateAttrs` - the rulings of rows 1, 13, 14 and 15, and the attribute content they read.
- The `locatororigin` registry in `internal/core/` - the originator, filled by the IS-IS plugin (B1).
- The receive-side withdraw of rejected NLRIs (I1).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be shown at implementation |
| No unintended coupling (components stay isolated) | No | the originator lookup goes through a registered resolver the IS-IS plugin provides, never an import of `internal/plugins/isis` from BGP (owner, 2026-10-11: "Yes, IS-IS resolver"). sysrib cannot answer it: `protocolRoute` in `sysrib.go` holds no originating router |
| No duplicated functionality (extends existing, does not recreate) | No | rows 1, 13, 14, 15 register on the check row 2 uses |
| Zero-copy preserved where applicable (refs, not copies) | No | to be shown at implementation |
| Registration over hardcoding, outbound | No | MUP checks live in the MUP plugin's registered check, not in the engine |
| Registration over hardcoding, inbound | No | to be shown at implementation |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The MUP per-NLRI recognizer can be given the UPDATE's Prefix-SID attribute without a central switch | `RecognizeNLRI` signature takes NLRI bytes only | Rows 1-2 need an engine-side hook that learns about MUP, which `ai/rules/principles.md` refuses | Read the engine call site of `RecognizeNLRI` | broken (2026-10-11): the recognizer cannot carry it; a sibling registry does, and it landed as `nlritype.UpdateCheck` |
| A-4 | GoBGP implements the MUP SAFI and can send a DSD route with and without a Prefix-SID | GoBGP release notes (unchecked here) | Rows 1-2 interop falls back to a ze-peer raw-hex functional test, and the interop gap is disclosed | Build the GoBGP container, send a DSD route | half holds (2026-10-11): with a Prefix-SID only |
| A-7 | `UpdateAttrs` can hand a check the Prefix-SID's SID and the MP next hop without copying on the receive path | `NewUpdateAttrs` wraps the attribute section "without copying it" (`update_check.go`) | The check needs a second view type | Read `NewUpdateAttrs`' caller at implementation | unvalidated |

- A-1 → Decision: `nlritype.Recognizer` answers about one NLRI alone, so giving it attributes would change its contract for every family. Rows 1-2 use a SIBLING registry beside it (per family, registered from the MUP plugin's `init()`): a check that takes one carved NLRI plus the UPDATE's path attributes and answers keep or treat-as-withdraw. The engine names no family; a family with no check is unaffected
- A-1 → Constraint: the draft says "A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message" (§3.3.6), so the verdict is per NLRI, and it is WITHDRAW, not the §5.4 discard: a DSD route stored earlier under the same key must leave the Adj-RIB-In
- A-1 → Constraint: the RFC 8669 §4 EBGP Prefix-SID discard (`session_validation_receive.go`) runs first. A DSD route from an EBGP peer not configured to accept the Prefix-SID therefore arrives at the MUP check with no Prefix-SID and is treated as withdrawn. AC-2 states it, and a functional test covers it
- A-1 → Constraint: the ISD rows `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1/-3/-4` carry the same `{gap}` and the same mechanism serves them (rows 13 to 15, in scope since 2026-10-21 because an absent requirement of an implemented draft is work to do, `ai/META-DECISIONS.md`); AC-9 carries them. An ISD rule is a further ruling on the check, not a second mechanism
- A-4 → Decision: GoBGP implements the MUP SAFI from v3.6.0, and the interop image pins v3.31.0 (`test/interop/Dockerfile.gobgp`). Its CLI DSD form (`cmd/gobgp/global.go::parseMUPDirectSegmentDiscoveryRouteArgs`) REQUIRES `prefix`, `locator-node-length`, `function-length` and `behavior` and always attaches an SRv6 L3 Service Prefix-SID, so GoBGP can send the stored case and a next hop of the operator's choosing, but never a DSD route without a Prefix-SID. The interop scenario proves the positive case and the keep-and-warn case against GoBGP; the without-Prefix-SID case (AC-2) and the proven-mismatch case (AC-1) are proven by ze-peer functional tests, and the scenario README says so. The README says the GoBGP CLI cannot send them (review N3): GoBGP's gRPC API takes arbitrary path attributes and could, so the limit is the CLI the scenario drives, not GoBGP. Before the scenario is written, the pinned version and the function are re-read in GoBGP's source (`ai/rules/interop-and-goal-validation.md`)

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-3 | Row 1 needs an IPv6 IGP lookup; with no IGP route for the locator Ze would withdraw every DSD route | DSD routes vanish in a lab without IS-IS | Owner decision on row 1 ("Keep it, log a warning"): an unresolvable originator keeps the route and logs a Warn; only a proven mismatch withdraws |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | MUP DSD or ISD routes withdrawn that should have been kept, or the reverse |
| How is it reverted? | Each row lands as its own commit, revertible alone |
| Who else touches this path? | `plan/release/fixes/spec-srv6-bestpath-resolvability.md` (same Prefix-SID read in the RIB candidate filter) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE with DSD route, no Prefix-SID, from a peer | → | MUP check treat-as-withdraw | `test/plugin/mup-dsd-without-prefix-sid-withdraw.ci` (exists) |
| UPDATE with DSD route whose next hop differs from the locator originator | → | MUP check, `locatororigin.Resolve` answered by the IS-IS resolver | `test/plugin/mup-dsd-nexthop-locator-mismatch-withdraw.ci` |
| DSD route whose locator originator the IGP table cannot resolve | → | MUP check keeps the route and logs a Warn (owner decision, row 1) | `test/plugin/mup-dsd-locator-unresolved-kept.ci` |
| DSD route from an EBGP peer not accepting the Prefix-SID | → | RFC 8669 §4 discard, then MUP treat-as-withdraw | `test/plugin/mup-dsd-ebgp-prefix-sid-discarded-withdraw.ci` |
| UPDATE with ISD route, no Prefix-SID, from a peer | → | MUP check treat-as-withdraw (row 14) | `test/plugin/mup-isd-without-prefix-sid-withdraw.ci` |
| UPDATE with ISD route whose Address or next hop the locator originator does not hold | → | MUP check, `locatororigin.Resolve` (rows 13, 15) | `test/plugin/mup-isd-originator-mismatch-withdraw.ci` |

## Acceptance Criteria

AC numbers are the numbers these criteria had in spec-bgp-unowned-rfc-rows.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (row 1) | A DSD route arrives whose MP_REACH global next hop is none of the addresses the originating LSP advertises, as the IS-IS locator-originator resolver answers it (the one system, across both IS-IS levels, whose SRv6 Locator TLV 27 longest-matches the Prefix-SID's SID with Prefix Attribute Flags present and R and X clear; its TLV 140, TLV 232 and host /128 TLV 236 addresses; the next hop compared per its length: 16 octets whole, 32 octets by the global half, 4 octets never in the set; Design resolutions, B1) | The NLRI is treated as withdrawn (removed from the Adj-RIB-In if stored, carried as an MP_UNREACH NLRI, never merely discarded), the other NLRIs of the UPDATE are processed, and a Warn log names the peer, the route, the next hop and the originator's system ID. A next hop equal to the TLV 140 address, to a TLV 232 address, to a host /128 in TLV 236, or to an IPv6 Source Router ID sub-TLV on a covering locator is stored (positive polarity, one case each). When the originator is not known, the route is KEPT and a rate-limited Warn names the peer, the route, the SID and the reason with its fix: no resolver registered; IS-IS engine not running in the daemon process (configure IS-IS, or run it internal); no advertised locator covers the SID; two or more systems advertise the covering locator with Prefix Attribute Flags present and R and X clear, a leaker not counted (anycast); any covering advertisement sets the A-flag (anycast, RFC 9352 §6), with a same-system TLV 236 Prefix Attribute Flags sub-TLV preferred over TLV 27 for R and A, and a same-system TLV 236 entry's external bit deciding X whether or not that entry carries the sub-TLV (round-5 I1); the covering locator re-advertised by another system (R or X set) with no Source Router ID equal to the next hop; the covering locator carrying no Prefix Attribute Flags sub-TLV (RFC 9352 §7.1); a Next Hop length other than 4, 16 or 32 octets; next hop unmatched while the LSP carries an undecoded TLV 237 (owner: "Keep it, log a warning"): only a proven mismatch withdraws |
| AC-2 (row 2) | A DSD route arrives in an UPDATE with no Prefix-SID attribute, including one whose Prefix-SID the RFC 8669 §4 EBGP discard removed | The NLRI is treated as withdrawn as in AC-1 and the other NLRIs of the UPDATE are processed; the same route with a Prefix-SID is stored. Landed and recorded for the unit and the accepting-peer `.ci`; owed here: the EBGP-discard `.ci` and the GoBGP scenario |
| AC-8 (ledger) | Each row once its AC is met | The row in `rfc/short/draft-ietf-bess-mup-safi.md` loses `{gap}`, carries tags in both polarities where Ze can observe the behavior, and carries a discrimination record (`./le rfc record`); `./le rfc check` exits clean for the stem |
| AC-9 (rows 13 to 15) | An ISD route arrives with no Prefix-SID attribute (row 14), or with one whose locator originator, resolved as AC-1 resolves it, does not hold the ISD Address field (row 13) or the next hop (row 15) | Per §3.3.3, the "Treat-as-withdraw procedure of [RFC7606] is applied: the NLRI is treated as withdrawn as in AC-1 and the other NLRIs of the UPDATE are processed; a matching ISD route with a Prefix-SID is stored (positive polarity). Registered on the same check as rows 1 and 2, never a second mechanism (A-1). AC-8 then applies to each row |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with a MUP speaker that sends a malformed DSD route | wire -> MUP check -> treat-as-withdraw -> no RIB entry | `test/plugin/mup-dsd-without-prefix-sid-withdraw.ci` |
| 2 | Peers with GoBGP, which sends well-formed DSD routes | wire -> MUP check -> stored, or kept with the Warn when no IGP resolves the locator | `bgp-mup-dsd-gobgp` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMUPDSDWithoutPrefixSIDWithdrawnOnReceive`, `TestMUPDSDWithPrefixSIDKeptOnReceive` (the plan named `TestMUPDSDWithoutPrefixSIDTreatAsWithdraw`) | `internal/component/bgp/reactor/draft_ietf_bess_mup_safi_dsd_prefix_sid_test.go` | AC-2 both polarities, the EBGP discard case included | landed, recorded |
| `TestMUPDSDNextHopLocatorMismatch` | `internal/component/bgp/plugins/nlri/mup/rfc7606_test.go` | AC-1 both polarities, and the unresolvable case; per Next Hop length: 16 octets compared whole; 32 octets compared by the global half (a Link-Local half equal to a set member does not match); 4 octets a proven mismatch against a known originator and kept with the Warn against a not-known one; any other length not-known | |
| `TestMUPDSDUnresolvedOriginatorKept` | same | AC-1 keep-and-warn | |
| `TestMUPISDWithoutPrefixSIDTreatAsWithdraw` | same | AC-9 row 14, both polarities | |
| `TestMUPISDOriginatorMismatch` | same | AC-9 rows 13 and 15, both polarities each, and the not-known keep | |
| `TestLocatorOriginResolveArms` | `internal/core/locatororigin/` | AC-1: no resolver registered answers not-known with its reason (never a zero address or an empty set), a registered resolver's known and not-known answers pass through with their reasons, a resolver error is returned as an error | |
| `TestISISLocatorOriginator` | `internal/plugins/isis/` | AC-1: the SID longest-matches the advertised TLV 27 locators (a /48 and a /64 both covering it pick the /64's LSP); the answer set holds that LSP's TLV 140, TLV 232 and host /128 TLV 236 addresses and no others; no covering locator, two systems on the covering locator (anycast), and an unmatched next hop with TLV 237 present each answer not-known with their reason; multi-level (round-3 B1): an L1L2 router advertising the locator in both its Level 1 and Level 2 LSPs, and across two fragments, is one originator, not anycast; a Level 2 copy with R set beside the Level 1 original resolves to the Level 1 system; a leaked locator alone, with R set and again with X set, answers not-known "re-advertised" naming the advertising system ID; the same leaked locator with an IPv6 Source Router ID sub-TLV equal to the next hop answers a known match; a covering locator with no Prefix Attribute Flags sub-TLV answers not-known with its reason; anycast by A-flag (round-4 I1): PE1 with R clear in Level 1 and only an R-set leaked copy of PE2 carrying the A-flag answers not-known anycast, not PE1; flag precedence (round-4 I1): a system whose TLV 27 flags have R clear but whose TLV 236 entry for the same prefix carries a Prefix Attribute Flags sub-TLV with R set is not the originator, and the TLV 236 sub-TLV's X-Flag is ignored (TLV 236's external bit decides X); external bit with no sub-TLV (round-5 I1): TLV 27 flags present with X clear, and a same-system TLV 236 entry for the same prefix with the external bit set and no Prefix Attribute Flags sub-TLV, answers not-known "re-advertised" naming that system, and the route is kept; with no published engine handle (engine not running, or external) it answers not-known "IS-IS engine not running in the daemon process" | |
| `TestISISWarnIfExternal` | `internal/plugins/isis/` | R2-I1: started external, the engine logs the one start Warn naming the DSD check and the fix and publishes no handle; started internal, it logs nothing and publishes the handle | |
| `TestSynthesizeWithdrawNLRI` | `internal/component/bgp/message/rfc7606_withdraw_test.go` | AC-1, AC-2: the body holds no Withdrawn Routes, no NLRI and exactly one MP_UNREACH_NLRI carrying the given AFI/SAFI and NLRI, flags Optional, Extended Length set exactly when the value exceeds 255 octets (value 255 and 256). Owed only if the audit of the landed withdraw path keeps this builder | |
| `TestRejectedDSDWithdrawBodyDispatched` | `internal/component/bgp/reactor/session_validation_nlritype_test.go` | AC-1, AC-2: an UPDATE holding kept and rejected DSD NLRIs dispatches the rewritten UPDATE (rejected NLRIs gone from MP_REACH, an MP_UNREACH of another family byte-identical) and the withdraw of the MUP family; the same on the treat-as-withdraw arm; when every DSD NLRI is rejected the withdraw is still dispatched; no dispatched body holds two MP_UNREACH. The audit names the landed test that already proves each clause | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MP_REACH Next Hop length compared by the check | 4, 16 or 32 octets | 32 | any other length is not-known and the route is kept | 33 and above: not-known, kept |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mup-dsd-without-prefix-sid-withdraw` | `test/plugin/` | AC-2 | landed |
| `mup-dsd-ebgp-prefix-sid-discarded-withdraw` | `test/plugin/` | AC-2 EBGP discard | |
| `mup-dsd-nexthop-locator-mismatch-withdraw` | `test/plugin/` | AC-1 | |
| `mup-dsd-locator-unresolved-kept` | `test/plugin/` | AC-1 keep-and-warn | |
| `mup-isd-without-prefix-sid-withdraw` | `test/plugin/` | AC-9 row 14 | |
| `mup-isd-originator-mismatch-withdraw` | `test/plugin/` | AC-9 rows 13, 15 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-mup-dsd-gobgp` | `test/interop/scenarios/` | GoBGP v3.31.0 (MUP since v3.6.0) | GoBGP sends DSD routes, always with a Prefix-SID (A-4): Ze stores them, and with no IGP route for the locator keeps them and logs the Warn. The absent-Prefix-SID and proven-mismatch cases cannot be produced by GoBGP's CLI and are proven by the ze-peer functional tests; the scenario README says so | |

The scenario owes the revert, rebuild, red, restore, green walk with the red recorded.

## Files to Modify
- `internal/component/bgp/plugins/nlri/mup/rfc7606.go`, `internal/component/bgp/plugins/nlri/mup/register.go` - rows 1, 13, 14, 15 (the MUP check's further rulings and their registration)
- `internal/core/bgp/nlri/nlritype/update_check.go` - `UpdateAttrs` gains the Prefix-SID SID and MP next hop the rulings read (A-7)
- `internal/component/bgp/reactor/session_validation_nlritype.go` - only where the audit shows the landed withdraw path does not yet cover a clause of I1
- `internal/component/bgp/message/rfc7606_withdraw.go` - `SynthesizeWithdrawNLRI`, only if the audit keeps it (I1)
- `internal/plugins/isis/register.go` and a new resolver file in `internal/plugins/isis/` - row 1: register the locator-originator resolver at `init()`; `runISISEngine` publishes the engine handle only under `p.IsInternal()` and calls `warnIfExternal` otherwise (R2-I1); the resolver takes the LSDB read lock, reads a locator index rebuilt on LSP change that spans Level 1 and Level 2 and keys advertisers by system ID, longest-matches the SID against TLV 27, walks the covering locators' sub-TLVs for Prefix Attribute Flags (type 4, including the A-flag, bit 4) and IPv6 Source Router ID (type 12), prefers a same-system TLV 236 entry's Prefix Attribute Flags sub-TLV, when present, for R and A, and takes X from that entry's external bit whether or not the sub-TLV is present, reusing or mirroring the X rule of `prefixAttributes` in `internal/plugins/isis/bgpls_export.go` (round-4 I1, round-5 I1, N1), and answers the originating system's TLV 140, TLV 232 and host /128 TLV 236 addresses (B1, round-3 B1)
- `internal/le/plugin/boundary/pluginboundary.go` - `locatororigin.Register` added to `dangerousCalls`, so the guard is checked (R2-I1)
- `internal/le/interoplab/bgp/check_special.go` - the `specialCheckers` entry binding `bgp-mup-dsd-gobgp` to its checker
- `rfc/short/draft-ietf-bess-mup-safi.md` - AC-8

## Files to Create
- `internal/core/locatororigin/` - the registry and its test
- the five new `test/plugin/*.ci` files the Functional Tests table names
- `test/interop/scenarios/bgp-mup-dsd-gobgp/` and its checker file with its `_test.go` under `internal/le/interoplab/bgp/` (route stored, keep-and-Warn log line)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no new leaf |
| YANG validation constraints | N-A | none new |
| YANG custom validators | N-A | none |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC; behavior `.ci` tests are listed above |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | the keep-and-Warn reasons are a problem class an operator hits (IS-IS not running in the daemon process, run external); the design names the doctor check and its code before implementation (`ai/META-DECISIONS.md`, "`ze doctor` reports every problem an operator can hit") |
| Prometheus counters/metrics | No | treat-as-withdraw counting already exists in the RFC 7606 path; confirm at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | MUP exists |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the `features/<id>.md` declaration whose evidence names the MUP rows (check `./le feature report`) |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, checked against the MUP check |
| 6 | Has a user guide page? | No | no MUP guide page; confirm through `ai/INDEX.md` |
| 7 | Wire format changed? | No | no new format; a withdraw is an MP_UNREACH_NLRI |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/draft-ietf-bess-mup-safi.md` and the generated `docs/features/rfc-status.md` (`./le rfc update`) |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | neither FRR nor BIRD implements MUP |
| 12 | Internal architecture changed? | Yes | the page `ai/CODE-TO-DOCS.md` names for `nlritype`, for the wider `UpdateAttrs` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the `locatororigin` registry and the IS-IS resolver registration: `docs/architecture/isis/isis-4-component-config.md`; the `dangerousCalls` entry: `docs/architecture/plugin/plugin-system.md` "The process boundary" where it lists watched calls |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/release/fixes/spec-bgp-mup-segment-discovery-originator-checks.md`; `docs/architecture/isis/isis-4-component-config.md` is declared by the IS-IS `register.go` and is affected; `rfc/short/rfc7606.md` is declared by `update_check.go` (Section 2 treat-as-withdraw, per route) and is unaffected, because the new rulings add no RFC 7606 behavior; `docs/architecture/core-design.md` is declared by files this spec changes and is unaffected: no change to the core and registration design, a new core-leaf registry of the shape section 19 already describes |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none |

## Implementation Steps

1. **Phase: Wiring** -- the new wiring `.ci` tests written and red; `/ze-audit` first, to mark what row 2 already built.
2. **Phase: row 2 proofs** -- the EBGP-discard `.ci` and the `bgp-mup-dsd-gobgp` scenario with its recorded red. Own commit.
3. **Phase: row 14** -- the ISD Prefix-SID presence ruling on the existing check. Own commit.
4. **Phase: row 1** -- the `locatororigin` registry, the IS-IS resolver, the originator comparison with keep-and-warn when not known. Own commit.
5. **Phase: rows 13 and 15** -- the ISD Address and next-hop rulings on the same resolver. Own commit.
6. **Phase: ledger** -- AC-8 for every row, discrimination records, `./le rfc check` clean for the stem.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Data flow | MUP checks stay in the MUP plugin; no MUP name in the engine |
| Correctness | A not-known originator never withdraws; each not-known reason is named |
| Rule: rfc-compliance | Each new implementing function quotes its draft sentence; each caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `{gap}` left on this spec's rows | `grep -n` each id in `rfc/short/draft-ietf-bess-mup-safi.md` |
| Both polarities tagged | `grep -rn "RFC requirement: <id>" internal/ test/` |
| Ledger clean | `./le rfc check` exit status for the stem |
| Process boundary guarded | `./le plugin boundary check` exits clean with `locatororigin` in `dangerousCalls` (R2-I1); the check is presence-based, so `TestISISWarnIfExternal` is the proof of the guard |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The checks parse peer-supplied bytes; bounds checked before reading the Prefix-SID |
| Resource exhaustion | The IGP lookup per DSD or ISD route must not block the receive path: an index read under the LSDB read lock, never a walk per NLRI and never an RPC |
| Error leakage | Warn logs name the peer and route, rate-limited through the existing per-route limiter |

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

## Owner decisions (carried from spec-bgp-unowned-rfc-rows)

| Question | Owner's words | What it means here |
|----------|---------------|--------------------|
| Row 1: when the locator's originator cannot be resolved from the IPv6 IGP table, is the DSD route withdrawn or kept? (2026-10-11) | "Keep it, log a warning" | AC-1: an unresolvable originator keeps the route with a rate-limited Warn; only a proven mismatch withdraws (R-3) |
| Row 1: where does the locator's originator come from, given sysrib holds no originating router (review B1)? (2026-10-11) | "Yes, IS-IS resolver" | A `locatororigin` registry in `internal/core/`, filled by the IS-IS plugin from the LSDB; BGP never imports IS-IS (B1) |
| Approve the design after the round-5 design review? (2026-10-11) | "Approve" | Covers rows 1 and 2 as written above |
| Split the source spec? (2026-11-14) | "split bgp-unowned-rfc-rows" | The MUP rows are this spec |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Rows 1-2 use a sibling per-family check registry beside `nlritype` | Widen `nlritype.Recognizer` to take attributes; an engine-side MUP hook | The recognizer answers about one NLRI alone for every family; an engine hook names MUP centrally (A-1) |
| Row 1 locator originator from a registered IS-IS resolver | Look it up through sysrib; import IS-IS from BGP | sysrib's `protocolRoute` holds no originating router; an import couples a BGP component to a plugin. Owner: "Yes, IS-IS resolver" |
| Rows 1 and 2 withdraw by dispatching the rejected NLRIs as their own withdraw-only body (round-3 I1) | Extend `rewriteMPNLRISections` to add an MP_UNREACH; a three-arm reactor edit that writes or extends an MP_UNREACH in the kept UPDATE | The first only shortens NLRI inside existing MP attributes; the second restated the Extended Length rule `message.appendMPUnreach` holds, reused that function's name, and needed a separate carrier when another family's MP_UNREACH was present. `SynthesizeWithdrawFamilies`' helpers and the `session_read.go` multi-body loop already do both jobs, so the rejected NLRIs take that path in every case (`ai/rules/principles.md`, declare once). The path that landed with row 2 is read first (I1, audit note) |
| The IS-IS resolver is a registry guarded by `.IsInternal()`, not a DirectBridge request and response (round-2 I1) | DirectBridge request and response to IS-IS; an event-fed originator index in the daemon | DirectBridge serves internal plugins only, so it adds nothing over the guarded registry and costs an RPC per DSD NLRI; an event-fed index copies the LSDB's locator and address sets onto the bus for one check. The guard makes the external case a not-known with a start Warn and a fix, and the boundary check enforces it |
| Row 1 compares the next hop with every address the originating LSP advertises (round-2 I2) | TLV 140 only | A PE's BGP next hop is often a loopback other than its TE Router ID; comparing TLV 140 alone withdraws a correct route, the opposite of the owner's keep-unless-proven decision |
| The originator is a system across both IS-IS levels, and only a covering locator whose Prefix Attribute Flags are present with R and X clear proves it (round-3 B1) | One LSDB level; the single LSP holding the covering locator | In a multi-level domain the Level 2 copy of a Level 1 PE's locator is the leaker's, and reading the leaker as the originator withdraws a correct route. RFC 7794 §2.1: "R-Flag: Re-advertisement Flag (Bit 1) Set when the prefix has been leaked from one level to another (upwards or downwards)." RFC 9352 §7.1 makes the sub-TLV a MUST only on a leaked or redistributed locator, and without it "receivers will be unable to determine the correct source of the advertisement", so an absent sub-TLV proves nothing. Consequence: a PE that omits the sub-TLV (RFC 9352 §7.1: it "SHOULD be included in the Locator TLV") never yields a proven mismatch, which the owner's keep-unless-proven decision accepts. A single R/X-clear advertiser is the originator because RFC 9352 §6 says "A prefix/SRv6 Locator that is advertised by a single node and without an A-flag MUST be considered node specific"; the A-flag on any covering advertisement, and a same-system TLV 236 Prefix Attribute Flags sub-TLV preferred over TLV 27, follow the same section (round-4 I1). X set with R clear makes the redistributing router the IS-IS advertiser; treating it as not-known is the conservative reading and keeps the route, as the owner decided |

## Known Limitations
- With IS-IS run as an external plugin process, or not configured, no originator is known and every DSD and ISD route is kept unverified with the Warn (R2-I1).

## RFC Documentation (Scope: protocol)

Add `// draft-ietf-bess-mup-safi Section X.Y: "<quoted requirement>"` above
enforcing code, using the quotes in the Task section, and the RFC 9352, 7794
and 5308 quotes above the resolver arms they decide.

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
- [ ] AC-1, AC-2, AC-8 and AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path` for each changed package (`./internal/component/bgp/plugins/nlri/mup/`, `./internal/core/bgp/nlri/nlritype/`, `./internal/core/locatororigin/`, `./internal/plugins/isis/`, `./internal/component/bgp/reactor/`), `./le test functional plugin` for the named `.ci` tests, and `./le test integration interop` for `bgp-mup-dsd-gobgp`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-mup-segment-discovery-originator-checks.md` only, in the same `./le commit create` script

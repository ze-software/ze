# Main-thread rulings (2026-09-29). Continuation authors apply these without asking.

## R1. A rule with no refusal path whose HEAD negative tag proves a neighbour
The coverage ratchet refuses {single-polarity} while a held negative exists. Do NOT add the marker.
Build a genuine negative instead (the RFC905-x-3 pattern): either (a) inject the non-compliant form at the RECEIVING side and assert it is refused/detected, or (b) force the producer's inputs toward the violation and assert the output still complies (and would not, under a targeted break). Keep the old negative tag only if it proves this row; if it proves a neighbour, MOVE it to that row (approval D-15) — this is allowed once the new genuine negative is in place (coverage stays held).
If neither (a) nor (b) exists, the verdict stays weak; add the id to OWNER-GATE list below.
Applies to: RFC5880-6.1-3, 6.8.1-4 (bfd); RFC7296-3.1-9, 3.14-3, 3.15.1-2 (ike); RFC1195-4.4-2 (isis); similar.

## R2. Row binds a role Ze does not fill, but the row holds tags at HEAD
(e.g. RFC1350-5-2 TFTP client; RFC2866-4.2-1 accounting server; RFC7950-8.3.1-1 NETCONF server; RFC7854-5-3 L-flag both states)
If the tags prove a different row's obligation -> move them there (approval), then retire this row under D-2/D-10 wording "binds <role>, which Ze does not implement" with the § cite. If the tags prove nothing Ze-owned, retire the row the same way and drop the tags. Retire paragraphs are gate-accepted. This is the route; not an owner question.

## R3. Absent feature inside a tagged row -> split the clause into its own {gap} row; the old row keeps its tests
(RFC9830-2.4.2-4 done; RFC5036-2.6.1.2-2 transit LSR label distribution; RFC5036-2.5.3-7 passive role done; RFC2661-5.8-8 retransmit count config).

## R4. Defects (D-8): fix, failing test first, with the recommended design
- L2TP unknown M=1 IETF AVP (RFC2661-4.1-3/4.1-4/24.12-1): single check in the AVP iterator beside FlagReserved; a Mandatory unknown IETF AVP aborts the message per §4.1 (session CDN for session messages, StopCCN for control); HELLO bodies parsed through the same iterator.
- RSVP-TE Resv state never times out (D-1, RFC2205-2.3-1): cleanupTick expires the reservation via removeReservation on non-egress LSPs; send what RFC 2205 §3.1.x says on RSB timeout (read it; ResvTear if stated).
- VRRP Backup forwards VMAC frames (RFC9568/5798/3768 6.4.2): nft ingress drop on the macvlan while Backup (same firewall route as the owner filter).
- BFD ReleaseSession deletes immediately (RFC5880-6.8.1-14): keep the entry AdminDown until LastReceived()+DetectionInterval(), delete on a tick.
- BFD no Poll on Down entry (RFC5880-6.8.3-2): follow the RFC; start a Poll when DesiredMinTx changes, including on leaving Up.
- sFlow counter polling overshoot (x-14/x-26): poll when elapsed >= interval - one tick; x-31 sub-agent-id: config validator refusing two collectors with different sub-agent-id on one data source.
- PPP RFC1661-4.3-2: after Terminate + Restart expiry, stay in Stopped able to take a new Configure-Request for the L2TP session's lifetime (do not end the session on Stopped); end only on L2TP teardown.
- BMP RFC7854-4.9-1 FSM event number for reason 2: carry the event in rpc.StructuredEvent from the reactor and write it in peerDownFor.
- RFC9830-2.1-3 SR Policy withdrawals lacking ORIGIN/AS_PATH: verify at cmd/update + reactor; fix if confirmed.
- YANG goyang structural checks (7.19-1, 9.4.4-1, 9.6.4.2-1 last sentence): move to plan/pre-release/spec-config-yang-loader-structural-checks.md acceptance criteria as Blocked by (P-3).
- LDP marks session operational before peer KeepAlive (journal row exists): if a verdict in scope depends on it, fix it.

## R5. Row rulings
- RFC7950-9.3.2-1: retire; tags move to a new §9.3.1 lexical row. RFC7950-8.3.1-1: retire per R2, tags to gap row 8.1-1.
- RFC2347-x-2: merge into x-4. RFC5176-3.5-2 vs 6.1-1 duplicate: merge. RFC3748-4.1-11 fold into 4.1-5/2.1-3; 7.10-1/7.10-2 merge. RFC2661 duplicates (10-2=24.10-1, 24.12-1=4.1-3+4.1-4, 6.12-1=10-1, 4.4.1-2=4.1-2): merge, keeping every tag.
- RFC2866-5-1 (quotes whole table): split per table row that states an obligation.
- RFC3579-3-1 NAS-Port SHOULD: send the attribute if Ze has a port notion (L2TP session id), else split + gap.
- Format/vector rows with no keyword (RFC2759 x-3/x-10/x-12, RFC3748-4-2): keep MUST under D-3.
- RFC5443-4-1 lowercase should: judge's D-3 call.
- RFC7854 x-3 split rows: per-section rows for §4.3, 4.5, 4.7, 4.8, 4.10.

## R6. Cross-package verdicts: the child that owns the stem writes the test in the other package when no other agent holds that file; otherwise queue after it.

## OWNER-GATE (collect; ask once at the top of a status)
- (none yet beyond R1 fallbacks)
- RFC7296-2.23-2 (R1 fallback: no genuine negative; either side may choose encapsulation)
- RFC7296-3.5-6 (gate refuses dropping HEAD tags; only receive refusal provable; Ze never sends ID_RFC822_ADDR)
- RFC4301-4.1-9 (R1: "without prejudice" has no genuine negative; the negative proves 4.4.1-10)
- RFC5880-6.1-3 (R1: obligation binds the pair of systems; Ze cannot see the peer role; no refusal path without new code)
- RFC5880-6.8.6-15 (R1: no genuine negative; the held AdminDown negative proves a neighbouring rule)
- RFC3787-x-2, RFC1195-4.1-1: RESOLVED by OWNER RULING 2 (negative = handling of absence).
- RFC3101-3.1-4 Nt clause (R31: Ze keeps a translate-never higher-ID router from wedging translation; ALSO Ze sets Nt on every non-never NSSA border router, RFC only for Always; recorded as a deviation; confirm)
- RFC4301-4.4.1-3 / 5.2-1 / 5.2-9 (R27: unmatched defaults to bypass; recorded as a deviation. NOTE 5.2-1 is a MUST, not only the 4.4.1-3 SHOULD: confirm keeping the default or flip it)

## R7 (2026-09-29). MUP 3.3-1, 3.3.7-2 bind the PE / MUP Controller roles Ze does not run (judge read RecognizeNLRI/typedNLRIEdit; no PE/controller function) -> R2: move tags that prove generic RFC 4760 negotiation to the RFC 4760 row they prove (if one exists), then retire both rows "binds the PE and MUP Controller roles, which Ze does not implement". 
## R8. Radius CoA: validate every Filter-Id/Disconnect value; apply all-or-nothing (given to radius continuation).

## R9 (2026-09-29). RFC1334-x-1: follow the RFC. When CHAP is supported, offer CHAP first; PAP only after a Nak; a configured PAP preference does not override the MUST. (Owner may interrupt: behaviour change for PAP-configured sessions.)
## R10. Independence breach: RFC1661-5.5-2 unit was edited by its judge (7758f5438e). The next ppp judge re-judges 5.5-2 independently.
## R11 (2026-09-29). RFC2865-3-4 vs the retired RFC2866-4.2-1: apply R2 consistently — if the quote binds the server generating the Response Authenticator, move tags that prove the client's verification to the verification/discard row (add it if missing) and retire 3-4. (radius cont 3)
## R12. MUP sibling {gap} rows binding PE/Controller (3.3.7-1, 3.3.8-1, 3.3.10-x, 3.3.11-1, 3.3.1-x, 3.3.4-x): out of this pass's scope (no weak/wrong verdict); changing them alters the public gap count — separate owner call, not done here.
## R13 (2026-09-29). RFC7854-4.9-1 (FSM event number for reason 2) is part of the reason-2/reason-4 close split owned by D1 of plan/immediate/spec-bmp-sflow-export-rfc-defects.md -> P-3: add to that spec's ACs + BGP child's Blocked-by; the bmp plugin/reactor change is done there, not here.
## R14 (2026-09-29). RFC3748-4-1: EAP Length > received octets is an EAP-layer error -> silent discard (RFC 3748 §4.1), distinct wire error, stay StateEAPInProgress, no notify; RFC 7296 §3.10.1 INVALID_SYNTAX covers malformed IKE messages, not EAP contents. Decrypt failure -> StateDead: journal row only.
## R15. RFC4301-5.1-2, 5.2-5: conditional on nested-SA re-crossing; Ze configures no nested SAs (neither in Go nor via XFRM bundles) -> retire citing the RFC condition; coverage held by RFC4301-4.4.1-11/12.
## R16 (2026-09-29, supersedes R4's PPP shape). After a peer Terminate-Request + Restart expiry, signal This-Layer-Finished (RFC 1661 §4.4 tlf: the lower layer is no longer needed) so the L2TP/PPPoE side starts its teardown, and keep answering a Configure-Request until the Down event arrives. No session may wait in Stopped without a bound. (ppp cont3)
## R17 (2026-09-29). RFC9568-7.2-1: the v3/IPv4 pseudo-header checksum is a documented, publicly disclosed interop deviation (keepalived; docs/features/rfc-status.md Partial, docs/architecture/vrrp/vrrp-first-hop-redundancy.md). R3 split: field-fill row (proven by the goldens) + checksum-clause row {gap} naming the deviation; move the tag off TestEncodeGoldenV3IPv4 to the field-fill row (D-15).
## R18 (2026-09-29). RFC9568-5.2.8-1: IPv4 receive also accepts the RFC 5798 checksum form. Same disclosed keepalived interop deviation as R17 -> same R3 split: the clause Ze meets stays on the tagged row, the deviating clause becomes a {gap} row naming the deviation, and the rfc9568 Support/Meta text discloses it where rfc-status.md already says Partial.
## R19 (2026-09-30). RFC7296-3.5-6 (ID_RFC822_ADDR terminator ban): Ze never emits an ID_RFC822_ADDR (encodeIKEID: IPv4/IPv6/FQDN only). The obligation is conditional on sending that ID type, an absent feature -> exclude as feature-out-of-scope quoting the §3.5 condition; the receive-refusal tags move to whichever row they prove, or stay as supplementary on 3.5-5 if that row covers FQDN receive.
## R20. RFC7296 §2.23 dynamic-update bound: add the row (verbatim sentence "When such a validated packet is found, ... they SHOULD dynamically update the address)."), tag TestNattUnauthenticatedPacketDoesNotMoveTheEndpoint to it and fix that test's comment (it cites the replay sentence).
## R21 (2026-09-30). RFC4301-4.4.2-1 (inbound SAD selectors). Judge the whole stack. Linux __xfrm_policy_check drops a decapsulated packet whose inner addresses match no inbound policy when its secpath holds a tunnel-mode SA (XfrmInNoPols), and drops one that matches another policy's template (XfrmInTmplMismatch). So in TUNNEL mode the negotiated selectors are enforced through the inbound require-policy Ze installs: not a defect. Prove it with the netns XFRM integration test (an inner packet outside the TS is dropped), and assert the policy Ze installs; do not assert x->sel. In TRANSPORT mode an unmatched packet with a transport-only secpath passes the policy check, so the selector must sit on the state: set SAParams.Sel from the negotiated pair (extend SASelector with ports), prove it the same way, and verify that MOBIKE/NAT-T migrate keeps x->sel consistent (read the kernel's xfrm_state_migrate or test it). Split a TS answer with more than one pair (which one x->sel cannot hold) into a {gap} row (R3). Retarget the untagged red test TestRFC4301InboundSADEntryCarriesTheNegotiatedSelectors to this ruling. Mirrors strongSwan: sel is set for transport, left any for tunnel.
## R22 (2026-09-30). RFC4301-4.4-1 is a framing sentence whose obligations are the §4.4.x rows: retire it (not-a-requirement, citing the rows that carry it), move its tags to the rows they prove (D-15). Not an owner question; count the retirement against D-7 (rfc4301 at 5/81 after it).
## R23. RFC4301-5.2-2 and 4.1-6 carry {not-applicable}: re-examine under rfc-compliance whole-stack rule; if Ze (or XFRM on its behalf) fills the role, drop the marker and prove what Ze installs; if a condition is truly absent, use feature-out-of-scope quoting the condition.
## R24 (2026-09-30). Ze DOES install multicast SAs: OSPF buildIPsecInterfaceSAs keys SAs to ff02::5/ff02::6 by hand, and RFC 4301 §4.4.2 says "The SAD can support multicast SAs, if manually configured." So RFC4301-4.1-6 stays, and the 4.1-8 retirement in 78ebfcce21 is wrong: restore 4.1-8 and judge both rows against the OSPF manual-SA path (the ospf package's tests, or new ones).
## R25. The DeriveRegister defect (plan/journal/exemption-still-billed-against-its-budget.md) blocks the goal: every split into unsourced-ids flips a stem's register. Fix the gate by subtracting unsourced-ids from gated before the comparison, failing test first, with the gates doc updated, in its own commit. Do not re-sign stems under prose to work around it.
## R26 (2026-09-30). RFC4301-4.4.2.1-3: the coverage ratchet refuses taking HEAD proof off a row. Keep the old tags on 4.4.2.1-3 (restore the deleted negative, keep the positive), ADD the 4.4.2.1-6 tag beside them on the same unit. The verdict stays wrong and blocked by spec-ipsec-lifetime-volume AC-7, whose implementation replaces the proof.
## R27. `vpn ipsec unmatched` defaults to bypass, so the RFC4301-4.4.1-3 SHOULD (a final discard entry) is unmet by default. Keep the default: flipping it drops every non-IPsec packet on an upgraded box. Record it as a disclosed deviation on the row and Support text, not counted conformant. Owner confirms at the final ask (OWNER-GATE).
## R28 (2026-09-30). RFC4301-5.2-1 (MUST discard a packet the SPD-I cannot match): Ze always installs a final catch-all SPD-I entry (bypass by default, discard with `unmatched discard`), so no inbound packet is ever unmatched; the MUST is met and the choice of the catch-all's action is the 4.4.1-3 SHOULD. Re-examine 5.2-1 (and 5.2-9) on that reading: if a netns probe shows an inbound packet always meets an installed entry (the catch-all), stamp enforced with that claim, and keep the deviation only on 4.4.1-3. If a path exists where no catch-all is installed (e.g. before config applies, or VPP), it stays a gap.
## R29 (2026-09-30). Two defects found by the crypto author, both D-8 (fix failing-test-first):
- The unmatched catch-all fails OPEN: installUnmatched's error is only logged (Warn) in applyConfig, and nothing is installed when the dataplane is nil / XFRM unsupported / before first apply. With `unmatched discard` configured, a failed install silently bypasses. Fix: a failed catch-all install fails the apply (error surfaced, config not reported applied); state what happens before the first apply and when XFRM is unsupported (refuse `unmatched discard` there). Then R28 is re-examined on the fixed code.
- engine lookupEncryption/lookupPRF/lookupIntegrity return a zero-value transform when the lookup fails, and config accepts 3des and chacha20poly1305 with no crypto registry entry, so ENCR 0 is offered with no error (ai/rules/principles.md: silently wrong value). Fix: the lookups return an error; config verification refuses an algorithm name the crypto registry does not hold (derive the accepted set from the registry, not a list). Update the IPsec config docs.
## R30 (2026-09-30). R29-b: take the error chain (option 1), not a panic. Config already refuses unregistered names, but a panic on the peer-setup path turns any future config source that skips verification (API, plugin) into a daemon crash. The lookups return an error; each of the 17 call sites propagates it, and a site with no error return gains one or resolves the transforms once at the boundary where the peer config enters the engine (preferred where it removes repeated lookups). Test with TestEngineProposalBuilderNeverOffersAZeroTransform (currently red on purpose).
## R31 (2026-09-30). RFC3101-3.1-2 (NSSA translator election). Two parts:
- Reachability: nssaABRs / nssaABRsV6 list border routers without checking they are reachable. That is a defect (D-8): list only NSSA border routers reachable in the area's SPF, and make the red TestRFC3101UnreachableBorderRouterIsNotListed green.
- Nt required for list membership: RFC 3101 §3.1 lets any reachable border router with a higher router ID disable a Candidate, so a higher-ID router configured translate-never would leave the NSSA with no translator. Ze counts a router only when Nt is set (TestOSPFNSSANonCandidateDoesNotWedge). Keep that behaviour as a disclosed deviation: R3-split the clause into its own row carrying the deviation (not counted conformant), disclose it on the Support text, and put it on the OWNER-GATE list for the owner to confirm.
## R32 (2026-09-30). RFC3623-5-2 (unplanned restart Grace-LSA before any Hello): split interface start into two steps — open and register every configured interface (fills e.running, no Hello yet), then originate the Grace-LSAs (maybeUnplannedRestart), then start the Hellos. Test: tag rfc3623_unplanned_cold_start_test.go and assert the first packet on the interface is the Grace-LSA LS Update, before any Hello.
## R33 (2026-09-30). RFC5881-6-1 defect: when resolveLoopDevices leaves a BFD loop unbound (sessions on several interfaces, or none named), UDP.Send ignores Outbound.Interface and a single-hop Control packet follows the route off its link. Fix: pin each single-hop packet to its session's interface in Send with an IP_PKTINFO / IPV6_PKTINFO ifindex control message; the red TestRFC5881UnboundLoopControlLeavesOnTheSessionLink goes green.
## R34 (2026-09-30). RFC8092-3-2 (RFC 8092 §3: "A receiving speaker MUST silently remove redundant BGP Large Community values"): remove repeats on the receive path in the same step where enforceRFC7606 already rewrites the payload, only when a duplicate is present (the common path stays zero-copy: scan first, rewrite only on a hit). §3 quote above the statement; the red TestRFC8092RedundantLargeCommunityRemovedBeforePropagation goes green.
## R35 (2026-09-30). RFC5709-3.2-2 (last key expiry): follow the RFC — §3.2 "treat the key as having an infinite lifetime until the lifetime is extended, the key is deleted by network management, or a new key is configured", and send a "last Authentication Key expiration" notification (a warning log + whatever operator event channel OSPF has). Ze currently refuses packets signed with an expired last key, which disrupts routing between two Ze routers: that is a defect (D-8), not a deliberate choice to keep. Apply it to both signing and verification. Split the row: the MUST NOT (no unauthenticated fallback) and the SHOULD (infinite lifetime + notification), each proven. Update the OSPF auth docs that describe the old refusal.
## R36 (2026-09-30). RFC2328-8.2-2 (OSPFv2 receive: source on the receiving interface's network). Implement the RFC's case 1 exactly: when the Area ID matches the receiving interface's area, compare source and interface address under the interface mask and drop on mismatch — on every network type EXCEPT point-to-point (§8.2: "This comparison should not be performed on point-to-point networks"); virtual links are case 2 and not subject to it. Test seam: give the engine the interface address and mask through the same path production config uses (no test-only field on the engine); fix the untagged rfc2328_receive_source_test.go setup so the mask is real, then +/- (broadcast mismatch dropped; point-to-point mismatch accepted). Not removable: it is a D-8 defect.
## R37 (2026-09-30). RFC5881-6-2 destination clause (single-hop peer must be on the interface's subnet): enforce at the one place every session passes — Send refuses a single-hop Control packet whose destination is on none of the egress interface's subnets (the interface addresses can change at runtime, so a config-time check alone is not enough), with a rate-limited log naming the session; also refuse it at config verify when a static single-hop session names an interface and a peer off its configured subnets. Test: the untagged red TestRFC5881ControlNeverAddressedOffTheSubnet goes green.
## R37b (2026-09-30). Drop R37's config-verify half. The runtime refusal in Send (plus the Warn log) is what enforces RFC5881-6-2 for every session, whatever its origin; a config-time check would need bfd to parse iface config and still could not cover DHCP/SLAAC addresses. Not worth a cross-component dependency.
## R38. RFC5882-4.2-1: under R6 the BFD child tags the two untagged BGP reactor tests (TestBFDClientAdminDownDoesNotTeardown, TestBFDRemoteAdminDownDoesNotTeardown in internal/component/bgp/reactor/peer_bfd_test.go) when no BGP agent holds that file.
## R39 (2026-09-30). RFC5303-3.2-10, 3.2-13, 3.1-6: implement the RFC 5303 §3.2 three-way state table in the IS-IS adjacency ReceiveHello (a new adjacency receiving three-way Up goes to the table's action, not straight Up). On Up -> Initializing, keep Ze's internal SessionDown notification: the adjacency leaves Up, so Ze must stop advertising it in its LSP (ISO 10589); the RFC's "no event is generated" is about the adjacency state-change event, not about withdrawing a non-Up adjacency. Say so in a code comment with both quotes.
## R40. RFC5036-2.6.1.2-2: no R3 split. The row is a MUST NOT that Ze meets by never mapping before the downstream label arrives; judge it on its test (the existing negative) plus a positive if one exists, else {single-polarity: negative} with the reason.
## R41. RFC5310-3.5-1: if the RFC wording requires the authentication failure be logged or reported, a Debug-level log is not visible by default — raise it to a rate-limited Warn (D-8).
## R42 (2026-09-30). OSPFv2 source check uses only the interface's first IPv4 address/prefix (Network()), so a neighbour on a secondary subnet of a broadcast interface is now dropped. Keep it: RFC 2328 gives an OSPF interface one IP address and mask (§9), and the check is §8.2's. Document it in ospf-4-component-config.md (only the first IPv4 address forms the OSPF interface; run a second OSPF interface for another subnet if supported, else say it is not).
## R43 (2026-09-30). BuildUnicast (bgp/message update_build.go) given an IPv4 prefix with an IPv6 next hop and no Extended Next Hop emits NLRI with no NEXT_HOP (reachable via `ze bgp encode`). Fix: BuildUnicast returns an error for that input; every caller (6) propagates it; the CLI reports it. The untagged red TestBuildUnicastIPv4RouteWithIPv6NextHopNeverLeavesWithoutNextHop goes green.
## OWNER RULING (2026-09-30, Thomas): RFC5880-6.1-3 — "we can not see the peer": no refusal path exists. RFC5880-6.8.6-15 — accepted as no genuine negative. Route for both: move each HEAD negative tag to the neighbouring row it actually proves (D-15), then mark the row {single-polarity: positive} citing this owner ruling; re-judge. (Next BFD slot.)
## R44 (2026-09-30). RFC3787-x-2 (Ze is always IP-capable; no refusal path for "TLV 129 present"): R3-split the §10 TLV 132 clause into its own row (verbatim, anchored), move the negative tag that proves it there (D-15), mark x-2 {single-polarity: positive} with that reason. Apply the same route to RFC1195-4.1-1 only if its audit note shows the same shape (no refusal path, negative proving a neighbour); otherwise work it normally.
## R45 (2026-09-30). RFC4271-8.2.2-7..17 FSM rows: test at peer level (the peer run loop owns the ConnectRetryTimer and resource release), not an R3 split — Ze performs those actions, so they are provable. Only clauses naming an event Ze does not have (e.g. Event 3 AutomaticStart in 8.2.2-7) R3-split into a {gap} row.
## OWNER RULING 2 (2026-09-30, Thomas) — what a negative is for a "must generate" / "must accept" row:
"RFC3787-x-2 we can only ever prove what we generate. RFC3787-10-1 same. RFC1195-4.1-1 that is something we can only prove on reception (not sending). The negative is how we deal with it when it is missing. If the operation of the protocol is impossible when we do not have it, we should follow the right way to deal with the absence (ignore, teardown whatever it is)."
Route: for a row obliging Ze to GENERATE something, the positive proves Ze generates it; the NEGATIVE proves how Ze handles a peer's PDU that lacks it (the RFC's prescribed handling: ignore, refuse adjacency, teardown...). If the RFC prescribes none and the protocol cannot operate without it, Ze follows the right way to deal with the absence. For a row obliging Ze to ACCEPT something on reception, the positive is acceptance and the negative is how Ze handles its absence/wrong form (RFC1195-4.1-1: refusing non-OSI encapsulation is that negative).
Applies to: RFC3787-x-2 (peer IIH/LSP without Protocols Supported TLV or without NLPID 0xCC), RFC3787-10-1 (peer IIH without TLV 132), RFC1195-4.1-1 (judge the existing refusal negative as valid). Candidate for the same reading: other generate/accept rows on the OWNER-GATE list.
## OWNER RULING 3 (2026-09-30, Thomas) — RFC5880-6.8.16-3 [MAY]: "The MAY is to allow different behaviour should the RFC wording be too strict. IMHO, we could decide to send 2x or 3x the minimum time. It leaves the decision to the implementer - us." Route: the MAY records Ze's chosen behaviour; the row is judged enforced on the test that pins that choice. Main-thread decision under it: Ze sends AdminDown Control packets for 3x the Detection Time (more robust to packet loss than the 1x minimum of 6.8.16-2), then stops; document the choice in docs/architecture/bfd.md and in the row note.

## OWNER RULING 4 (2026-09-30, verbatim): "yang is provided by a third party library we should not try to close the gap"
Owner approved the main-thread reading ("all correct"): rfc/short/rfc7950.md Meta becomes `mixed`, naming openconfig/goyang (module parsing) as implementer and Ze's validator for data checks; remaining open rfc7950 verdicts (7.6.5-1, 9.1-1, 8.1-2, 8.3-1, 8.3.3-1, and 7.6.1-1 if it cannot be tagged without new code) are recorded as {gap}s and moved into plan/pre-release/spec-config-yang-when-unique-choice.md acceptance criteria (P-3 style), not fixed. No new validator/YANG-module code. The red untagged validator_mandatory_rfc7950_red_test.go stays uncommitted, attached to that spec.

## OWNER RULING 5 (2026-09-30, "Recommendations lgtm"): OWNER RULING 2 applies to the six IKE/EAP owner-gate ids:
- RFC4301-4.1-9: judge enforced on the positive (parallel SAs over real XFRM); existing negative stays supplementary (BFD 6.1-3 precedent).
- RFC7296-3.5-6: enforced under ruling 2 (Ze never sends ID_RFC822_ADDR; receive refusal of a NUL-terminated peer ID is the negative).
- RFC7296-3.15.1-2: enforced under ruling 2 (Ze never sends CFG_REQUEST; it ignores a peer's non-empty netmask per §2.5).
- RFC7296-2.23-2: new negative: plain (non-UDP) ESP arriving on an encapsulated SA under NAT is not delivered.
- RFC5216-3-1: new negative: a first fragment without the L bit is discarded as malformed; fix the tag prose that claims a MUST NOT for middle/last fragments.
- RFC5216-2.1.5-1: new negative: fragments whose reassembly is inconsistent (TLS Message Length mismatch) are refused.
## R46 (2026-09-30, main thread; corrects my ruling-5 recommendation for RFC7296-2.23-2): the requested negative would assert a breach of RFC7296-2.23-10 ("all devices MUST be able to receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time"), which Ze meets on purpose (AcceptBothESPForms). So no receive-side refusal is legitimate. Judge 2.23-2 on its positive (NAT detected -> UDP encapsulation on the SAs Ze installs); the existing no-NAT negative stays supplementary (BFD 6.1-3 precedent).

## R47 (2026-10-01, main thread, after BGP c8 judge 07bdc7ca76): (a) RFC4271-8.2.2-20 (Event 3) stays a counted {gap} per R45: Ze has an automatic start (Event 6), so the AutomaticStart obligation is applicable and only the Event-3 form is absent; no owner scope decision declines it, so feature-out-of-scope is not available. (b) Forward-arithmetic misses (an RFC sentence Ze implements with no row, e.g. RFC4271 8.2.2 Active "any other event", Established and OpenConfirm ManualStop): ADD the row (verbatim span, extraction site mapped, dated correction) with tests both polarities and records, in the same continuation that finds it. (c) FSM.change callback-unlock defect (journal unguarded-goroutine-kills-the-process.md) is journal-only: not this goal.

## R48 (2026-10-01, main thread): RFC4271 §8.2.2 "any other event" in OpenSent/OpenConfirm/Established (sends FSM Error NOTIFICATION) has no row, and Ze likely sends none (UPDATE in OpenSent ends with ErrFSMError, no NOTIFICATION). spec-bgp-open-session-rfc-defects does not name it, so per R47(b) and the child Failure Routing it is THIS child's: next BGP author adds the row(s), writes the failing peer-level test first, fixes the D-8 with the RFC quote above it, docs in the same change.

## R49 (2026-10-01, main thread, after BGP c11 author)
(a) RFC4271 §8.2.2 Established "any other event (Events 9, 12-13, 20-22)": the Event-specific error procedures of §6.1 (header, Event 21) and §6.2 (OPEN, Event 22) are the more specific MUSTs and govern the Error Code; the §8.2.2 Established sentence still governs the teardown actions (delete routes, release resources, drop TCP, counter, Idle). Events 9, 12, 13 and 20 cannot occur in Ze's Established (no ConnectRetryTimer running, no DelayOpen, no IdleHoldTimer). So: ADD the row (verbatim span) with a correction paragraph stating this reading; its tests assert the teardown on Event 21/22 in Established and the §6.1/§6.2 code on the wire (claim states exactly that, not "FSM Error"); records.
(b) RFC4271-8.2.2-13 (Active + TcpConnectionFails, Event 18): reachable only with the optional DelayOpen feature (Ze goes Active->OpenSent at once on TCP establishment; fsm.md: no DelayOpenTimer, permitted by §8.2.1.3). DelayOpen is an OPTIONAL feature no agreed scope includes, so per rfc-compliance: record DelayOpen as an implementation gap in the rfc4271 Support text, and exclude the conditional obligation with excluded-kind feature-out-of-scope, reason quoting the RFC optionality and naming "no agreed scope includes DelayOpen; docs/architecture/behavior/fsm.md". Same treatment for any other row conditional only on DelayOpen or DampPeerOscillations.
(c) RFC 6608 subcodes stay gaps (no scope change); 5/0 is the correct code without RFC 6608.
Judge finding on R49(b) (BGP c12 judge, 2026-10-01): the exclusion cannot be expressed for 8.2.2-13 with today's gates. Row-level, the kind is `{feature-declined}` (rfc-conformance-gates.md: the coverage register's word for feature-out-of-scope), but `./le rfc check` refuses it beside the row's HEAD tags (check_core.go "annotated but IS tested"), and checkCoverageRatchet refuses removing those tags. The row is an unsourced id, so no extraction site exists to carry excluded-kind, and the RFC states the sentence, so retirement is closed. Applied: DelayOpen disclosed as an implementation gap on Support remaining; row left without annotation, verdict weak. Open for the main thread: a gate change (let `{feature-declined}` stand beside HEAD tags, or let the ratchet accept tags dropped from a declined row) if the exclusion must land.

## R50 (2026-10-01, main thread, after BGP c12 judge 1e5c829890): RFC4271-8.2.2-13. The gate refuses {feature-declined} beside existing tags. Read what each HEAD-tagged unit actually drives: if it drives a synthetic Event 18 into the FSM in Active (unreachable from a real Peer without DelayOpen), its tag claims proof of behaviour no layer performs, so the tag is a wrong claim: remove it (D-15 approval naming this ruling; the ratchet accepts an approved removal) or retag it to the row it does prove, then land the {feature-declined} annotation per R49(b). If instead a unit shows Event 18 reachable at peer level, the row is applicable: prove it both polarities and stamp it on evidence. No gate change.

## R51 (2026-10-01, main thread, after BGP c13 judge 36435ca55c)
(a) A {feature-declined} row carries NO audit verdict (services precedent dbdde9fb4f: RFC7011-7-1/7-2 have no entry in rfc/audit/rfc7011.json). RFC4271-8.2.2-13: remove its audit entry the same way (and its three orphan records naming untagged units), so it leaves the weak listing.
(b) RFC4271-5.1.2-2 fix incomplete: the guard must use the session's migration-aware IBGP verdict (facts.isEBGP / PeerSettings.isIBGPWith), not destPeerAS == destLocalAS, so an RFC 7705 internal-by-§4.2 session is covered; add that case as a test; make policy_dryrun.go computeWireChanges agree with the runtime. Doc pages follow.
(c) Records this orchestration staled by its own producer changes are owed by the next BGP author in the same stem pass: rfc8654 (readAndProcessMessage) and rfc8955 (comparePair) if git log shows our commits changed them.

## R52 (2026-10-01, main thread, supersedes R51(a)): the audit skill forbids hand-deleting an audit entry and audit-stamp cannot remove one. RFC4271-8.2.2-13 is an applicable-only-with-DelayOpen obligation Ze does not implement: stamp it unimplemented through audit-stamp (mode rejudge), note naming the {feature-declined} annotation and §8.2.1.3. Under AC-C3 an unimplemented stamp is a verdict and leaves the weak listing. Do not delete audit keys by hand.

## R53 (2026-10-01, main thread, after BGP c14 judge 1e54dbbc7d; supersedes R52)
(a) RFC4271-8.2.2-13: the gate accepts an unimplemented verdict only beside {gap} or {not-applicable}, and the ledger has no exclusion for an unsourced row. Replace the {feature-declined} annotation with a {gap} annotation (DelayOpen is OPTIONAL per RFC 4271 section 8.2.1.3 and not implemented; Event 18 in Active is reachable only with it), keep the Support text naming the DelayOpen gap, and stamp unimplemented. Counting it as a gap can only understate conformance, never overstate it. No gate change.
(b) RFC4271-6.3-2 "SHOULD be logged": a route ignored for a semantically incorrect NEXT_HOP must produce a log record visible at the default level (ze.log default Warn). Emit it at Warn, through an existing rate limiter if Ze has one for per-route diagnostics (a peer can send many), and prove it with a test at the default level. Doc page in the same change.
(c) RFC4271-5.1.2-2: add the forward-rail unit the judge named (forwardUpdateSection: export prepend/remove-private filter, internal peer AS_PATH unchanged, external control edited), record it.

## R54 (2026-10-01, main thread, after BGP c15 author): RFC4271-5.1.3-3 next-hop self with local ip auto on the forward rail sends the third-party NEXT_HOP. Fix: build next-hop self from the session's connected local endpoint (the TCP local address of the established session, the address the peer reaches Ze on), on every rail that honours next-hop self, so forward and announce rails agree; withhold the route with a warning only if no local address exists. Do NOT refuse next-hop self + local ip auto at config load: it is a valid, common configuration. The announce rail's ErrNextHopSelfNoLocal must then also use the connected endpoint when one exists. Failing test first (rfc4271_third_party_nexthop_red_test.go already exists, untagged), RFC quote above the fix, doc page.

## R55 (2026-10-01, main thread, after BGP c18 author)
(a) RFC 2545 section 3, third-party next hop on a shared link: Ze must never pair another router's global address with Ze's own link-local. Fix linkScope.linkLocalNextHop: append Ze's link-local only when the global next hop is Ze's own address. The third-party case (RFC requires the next hop's own link-local, which Ze cannot learn) becomes a {gap} row split from RFC2545-3-3 under R3 (verbatim span, dated correction, Support count). Failing test first: rfc2545_own_link_local_red_test.go is the red; fold it into tagged units after the fix. Doc page.
(b) RFC4271-9.2-7 (a newly selected best route is readvertised): probe the route-server relay first (bgp-rs: source A withdraws while source B's route for the same prefix remains; a client without ADD-PATH must receive B's route, not a bare withdrawal). spec-bgp-update-propagation-rfc-defects does not name it, so a confirmed defect is THIS child's D-8: failing test first, fix, RFC quote (RFC 4271 9.2 and RFC 7947 where it applies), doc page. If the probe shows no defect, prove 9.2-7 both polarities on the path that does readvertise.

## R56 (2026-10-01, main thread, amends R55(a) after BGP c18 judge 4755e4d1f6): the linklocal draft section 4 requires Ze's own link-local also when the route is reachable through the receiving internal peer's address (the global is the PEER's, not Ze's). So the R55(a) fix appends Ze's link-local when the global next hop is Ze's own address OR in the draft section 4 case (read the draft's exact conditions and quote them above the predicate); it is withheld only for a third-party global that is neither. Add the 4-3 unit the judge named (explicit next hop = the peer's address, Ze's own link-local included) with a record, in the same change.

## R57 (2026-10-01, main thread, after BGP c19 author)
(a) Linklocal draft 4-3 second condition (route reachable through the receiving internal peer's own address): Ze never advertises such a route, because RFC 4271 section 5.1.3 forbids a NEXT_HOP that is the receiving peer's address and Ze refuses it on every rail (owner decision 2026-08-15). Remove the now-dead internalPeer branch from linkScope.linkLocalNextHop (no rail reaches it; simplicity, no dead code). Split the second condition from 4-3 under R3 into its own row annotated {not-applicable: the antecedent is a route Ze is forbidden to send, RFC 4271 section 5.1.3 quoted}, proven by TestLinkLocalSecondConditionNeverReachesTheWire (asserts nothing reaches the wire); 4-3 keeps the first condition. Dated correction.
(b) RFC4271-9.2-7 route-server replacement on withdrawal: confirmed defect, fix needs a per-source path store in bgp-rs (feature-sized). Per child Failure Routing / P-3: home it in a new spec plan/immediate/spec-bgp-rs-replacement-on-withdrawal.md whose AC names RFC4271-9.2-7, add 9.2-7 to this child's Blocked by. The owner is asked whether that spec runs now. The red probe internal/component/bgp/plugins/rs/rfc4271_replacement_red_test.go moves to that spec (never committed here).

## R58 (2026-10-01, main thread, after BGP c19 judge 771fc01602)
(a) Capability 77 (link-local next hop) is advertised without a configured `session > link-local` address, so Ze can negotiate it and then never send its own link-local (breaks linklocal 4-3 and RFC2545-3-3). Fix: refuse at config load enabling the capability on a peer that has no link-local address configured, with an error naming the missing leaf (simplest fully correct: deterministic and testable; deriving it from the interface adds a hidden runtime dependency). Failing test first (config validation + the wire consequence), doc page, records.
(b) RFC2545-3-6 {gap} text and Support remaining overreach: the gap covers only the rails where Ze writes the next hop; with next hop unchanged a received 32-octet field is relayed as is. Correct the wording (dated correction).
(c) Linklocal extraction site 1:2 ("implementations must ensure ... strictly associated with a specific interface") binds Ze: add the row (verbatim span, both polarities or {gap} if Ze does not do it), map the site, so the extraction can be re-signed. Sites 1:1, 1:3 and front:1: map or exclude with reasons.

## OWNER RULING 6 (2026-10-02, Thomas: "i am fine with your recommendations")
(a) Orphan discrimination records (a record whose tag moved off that unit, or was removed, while the row keeps a recorded unit in that polarity) are deleted. Approved: the 7 RFC5880 orphans the BFD judge deleted, and RFC3748-5.4-1 -, RFC3748-5.4-2 -, two RFC4301-4.4.2.1-1 records on rfc4301_boundary_linux_test.go, RFC4090-4.4-7 - on TestRFC4090NodeBitClearWithoutNodeBypass, RFC1195-5.2-2 on TestISISTLVIPv4InterfaceAddr. No `./le rfc` verb deletes one, so deletion is a scripted JSON edit, verified with `./le rfc discriminate id` afterwards.
(b) The coverage ratchet (internal/le/rfc/check_ratchets.go checkCoverageRatchet) gains an exception for an owner-ruled tag move: a polarity lost against HEAD is accepted when the commit's D-15 approval for that unit cites an owner ruling and the row now carries a {single-polarity} annotation citing the same ruling. Failing test first, gates doc updated, own commit. Then apply the 2026-09-30 ruling on RFC5880-6.1-3 and 6.8.6-15.

## OWNER RULING 7 (2026-10-02, Thomas, RFC4724-4-4): "we should allow this for all family for which the Ze configuration has a RIB and can re-send the routes, so it depends on the plugin loaded"
The Graceful Restart Capability lists an <AFI, SAFI> only when the running configuration has a RIB holding that family's routes that can re-send them after a restart, which depends on the plugins loaded. Derive the set from what the loaded plugins register for the family (registry, never a hand list). A family no loaded RIB holds is left out; when none qualifies, the capability is still sent with no tuples (RFC 4724 section 4 recommendation, keeps End-of-RIB). An operator-configured `graceful-restart family` list is intersected with that set, never widened past it: a configured family no loaded RIB holds is dropped with a Warn naming the peer and family (never silently), and the guide says so. The F bit keeps its existing rule (set only when the forwarding plane kept its routes).
## OWNER RULING 8 (2026-10-02, Thomas: "ok" to the main thread's recommendation table)
(a) Ruling 7 is built from the RIB's splitter registry: a family is listed in the GR capability only when the RIB plugin can store it (nlrisplit.Supported) and the RIB plugin is loaded; today every compiled-in family qualifies, so only runtime-registered external families drop (Warn). No new Registration field.
(b) The RIB dropping a negotiated family with no splitter at Debug level is a silently-wrong path: journal row now, fix in a later pass.
(c) ADD-PATH best path per (path id, prefix) (RFC8277-3.1-1, 2.5-2): home it in a new spec; both ids go to the BGP child's Blocked by. Whether it runs now: ask the owner when the spec exists.
(d) RFC8277-2.2-3 transmit: clear the Rsrv bits on relay, rewriting only when a non-zero Rsrv bit is present (scan first; the common path stays zero-copy, R34 precedent); RFC 8277 2.2 quote above the statement.
(e) Per-AFI/SAFI LLGR configuration (RFC9494-5-2): its own small spec; the row stays {gap} until it lands.
(f) RFC4302-3.4.3-5: {single-polarity: positive} citing this ruling ("MUST support 32" has no violating input; the 1/31 refusal is Ze policy); the HEAD negative tag leaves the row under D-15 citing owner ruling 8 (coverage-ratchet exception, ruling 6(b)).
(g) RFC2328-8.1-1: unnumbered point-to-point is an absent feature: record it as a {gap} (implementation gap on the Support text); remove the tag from the test that asserts the refusal of an address-less interface (D-15, owner ruling 8) or retag it to the row it proves.
(h) RFC8665-5-14 (SHOULD PHP): follow the RFC (PHP in the listed cases, including for an M-flag SID), after reading FRR's decision and quoting it; failing test first.
## R59 (2026-10-02, main thread): RFC9494-5-1 is this child's D-8 (spec-bgp-graceful-restart-rfc-defects D1..D5 do not name it): a received LLGR capability (code 71) is honoured for a family only when Ze configured LLGR for that peer and family (RFC 9494 section 5 "MUST require affirmative configuration per AFI/SAFI"). Red test exists: plugins/gr/rfc9494_helper_default_red_test.go. Fix docs/guide/graceful-restart.md "only active when both peers negotiate it".

## Owner decisions (2026-10-02 continuation ba93202e)

Thomas answered the eleven pending questions after the stale-record repair:

1. Authorize the main thread to remove only the retired RFC8955-6-2 audit entry.
2. Authorize reconciliation of the RFC8955 extraction exclusions and renewal of its sign-off after checking the recorded retirement; no unperformed walk may be claimed.
3. RFC5575-6-1: disclose the gap and RFC9117 section 7 optionality; do not implement the optional check.
4. ROUTE-REFRESH: retain config-static routes in ribOut with an origin flag; refresh includes them, peer-up replay skips them.
5. VPP SRv6 policy installation: separate pre-release spec, with a local binding SID allocated from a local locator. Preserve the red test and disclose the missing behavior.
6. Duplicate Prefix-SID types 5/6: add the defect and red test to spec-bgp-prefix-sid-rfc-defects acceptance criteria under P-3.
7. Label-Index Reserved and Flags: clear them on transmission, including relay. Preserve Originator SRGB unchanged. This corrects HANDOFF.md: RFC8669 section 3.2's unchanged-propagation sentence binds Originator SRGB, not Label-Index.
8. Run spec-bgp-addpath-best-path-per-prefix after this pass and before release.
9. LLGR configuration: one-shot migration to explicit per-family intent, preserving the advertised OPEN behavior; remove the old shape.
10. RFC9514-7.2-5/6: authorize a scoped clause-aware ledger representation that retains the RFC sentence, receive proof and explicit origination gap. Do not mark the whole obligation enforced.
11. RFC5082-3-3: local-output proof only, with genuine configured egress evidence. Authorize single-positive coverage and removal of the misleading negative tag; retain the calibration test without that tag.

The other two handoff items need no new ruling: R3 already governs the RFC9086-7-1 PeerNode/gap split; the RFC2866 Linux recording is being attempted through the existing local QEMU route. No push is authorized.

### Clause-ledger design approval (2026-10-02)

Thomas chose **Annotation plus partial verdict** for decision 10. The approved
contract is `plan/pre-release/spec-rfc-clause-scoped-proof.md`: one tested span
and one explicit gap span under the complete RFC requirement; scope-sensitive
audit freshness; existing test and record identities retained; partial earns
zero whole-requirement proof credit. Independent judgment remains mandatory.
The alternative requiring separate clause identities and record migration was
not chosen. Native SRv6 BGP EPE origination remains outside this work.

### MSS retirement and EVPN polarity (2026-10-02)

Thomas chose **Retire obsolete MSS row** for RFC2385-4.3-1. RFC6691 Section 3.2
explicitly corrects that sentence; RFC9293 Section 3.7.1 retains fixed-header
advertised MSS and option-aware data sizing. Authorize its retirement, removal
of its audit entry and orphan records, and removal or reassignment of misleading
tags. Keep packet proof of current behavior; do not enroll all of RFC9293 merely
to assign that proof an id.

Thomas chose **Authorize single-positive coverage** for RFC7432-8.2.1-9.
Move label-validity tags to the requirement they prove and retain genuine
route-support positives, with the D-15 approval citing this ruling. This approves
the polarity correction, not an enforced verdict: an independent judge must
still establish what support of the route requires and what Ze proves.

### MRT mixed-family context (2026-10-02)

Thomas chose **Implement context-aware parsing now**. RFC8050's whole-message
subtype does not encode the per-family modes negotiated under RFC7911. Recover
those modes from actual captured directional OPENs and session lifecycle;
when that evidence is unavailable, report ambiguous decoding instead of
publishing incomplete route totals as complete. Preserve captured message
bytes: no invented Path Identifiers, synthesized OPENs or split UPDATEs.
This work runs in the current BGP child's MRT phase, not a deferred spec.
Unsupported ADD-PATH replay must refuse before sending; a new negotiated
replay feature is not authorized by this decision.

### Scoped duplicate and test cleanup (2026-10-02)

Thomas authorized deletion of only the retired RFC9086-5-5 audit entry and
its two orphan discrimination records after all three tags moved to the
canonical RFC9086-5-3. The retirement explanation and migrated proof remain.
He also authorized removal of only the newly added
`TestLLGRCommunityCommandsAreDeclared`: the real `llgr-import-no-llgr`
daemon workflow exercises both commands. The existing command-summary test,
workflow assertions, stress evidence and RFC records remain unchanged.

### RFC2385 no-response proof (2026-10-02)

Thomas chose **Move the claim to packet capture** for RFC2385-2.0-3.
Remove only the misleading timeout-only tag from
`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`; keep its executable
assertions. Bind the requirement to direct malformed-signature packet
observation with an independently signed valid control, then record and
independently judge that evidence. A failed keyed dial cannot itself prove
that the peer sent no response.

### LLGR sent-inventory ownership (2026-10-03)

Thomas chose **Generalize Adj-RIB-Out keys**. Extend the existing sent inventory
and replay path to registered opaque NLRI identities as well as prefixes;
RIB owns retained attributes and eventual source-specific withdrawals.
Do not add a second RS lifecycle inventory. Preserve the existing CIDR fast
path, exact native framing and ADD-PATH identity, including identifier zero.
Source ownership belongs to each destination's entry rather than one global
source per family/key. This authorizes the broader sent-store/replay/show
prerequisite for the four real LLGR workflows, not the separately scoped
GR/LLGR defects D1–D5.

### Owner-directed commit and pause (2026-10-03)

Thomas requested: "commit all work and pause when you reach the right point
please", then "commit logically in groups". This is a checkpoint, not acceptance
or closure of the parent or BGP child. Both specs remain open. No push is
authorized and no full-worktree green result is claimed.

Evidence below is under
`tmp/session/2026-10-02-ba93202e-6f62-48a4-9b4e-c0aa37a4cc74/scratch/`.

| Surface | Observed result and remaining obligation |
|---------|------------------------------------------|
| Failed-await diagnostics | `job-stable-lifecycle-proof-d0d75fa5.log`: both fence forms and unfinished lingering peer save all four streams, received frames and shutdown-only bytes under `-race`. Independent source review accepted the repair. |
| GR/LLGR stored state | The same log passes all seven migrated RFC4724/RFC9494 carriers, including no-capability deletion and serial GR/LLGR expiry. Native records for changed claims still need renewal; this is not four-workflow acceptance. |
| MRT lifecycle | The same log passes the original live collision and deterministic third-arrival `published`/`taken` reservations. Independent review accepted C1/C2/C3 and the reservation repair. RFC8050 x-1/x-4 remain weak because the decisive existing session, boundary and capacity proofs still need accurate tags and native discrimination links, not because those implementations or tests are absent. |
| EVPN | The same log passes the repaired IPv4/IPv6 sender with mandatory Label1. The single-positive ruling is independently defensible for the declared BGP control-plane role. Five native records, audit stamps and the shifted-row reseal remain owed; no full EVPN PE claim is made. |
| Generic forwarding | `job-generic-forward-policy-proof-499402fc.log` is red under `-race`: `TestRFC7947AllAttributesReachClient` and `TestRouteServerTransparencyStopsAtOrdinaryPeer` expose startup synchronization races, including `SetAPIProcessCount` against `signalStartupComplete`. Mixed-treatment cache assertions pass. Independent review also found avoidable payload allocation in `forwardWire`; neither finding is fixed at this checkpoint. |
| Accounting | The UDP deadline regression passed in `job-accounting-fixture-smoke-919d5de0.log`; its repair is `e39d58afe7`. Native `rfc discriminate-record` completed on QEMU Linux 7.2 in 757.53 seconds and wrote RFC2866-4.1-1 positive for `test/l2tp/radius-acct-wire.ci`. The clean workflow passed; disabling `onSessionIPAssigned` produced the observed `Accounting-Start did not arrive within 30s` failure. The recorded citation is the IPCP-negotiated address `10.99.7.10`. |

Resume the existing scope, without rerunning unchanged successful checks:

1. Resolve the observed startup races from their complete race stacks. Apply
   generic opaque treatment within owned pooled materialization, and include
   effective treatment in `fwdDedupIdentity` before deduplication. For shared
   unmodified input, use the existing read-buffer/adopted-handle lifetime.
   Preserve mixed-role, absent-plugin and inverted-selector wire assertions.
2. Use the repaired runner to save the real `llgr-rib-stale` dialogue before
   correcting its peer expectations. Run the family-scope and explicit-port
   repairs in `llgr-readvertise` and `llgr-peer-stale-time-drives-timer`; finish
   all four workflows and their required stress proof before promotion.
3. Enroll the already-reviewed MRT session/boundary proofs, renew changed GR,
   RIB and forwarding records, and finish the EVPN native commands recorded in
   `evpn-proof-landing-commands.txt`. Native stamp/reseal writes remain serialized.
4. Complete the remaining BGP row inventory and independent judgments; the
   protected red probes and previously named defect specs retain their scope.
5. Finish child/parent review, goal validation and full-worktree verification.
   Do not close either spec merely because this checkpoint is committed.

The four unpromoted LLGR drafts remain in ignored `test/draft/plugin/`, not in
the live suite. Their exact checkpoint bytes are also preserved in
[`llgr-drafts-2026-10-03.patch`](llgr-drafts-2026-10-03.patch), with original paths
in each patch header. This records the requested work without promoting
unproven tests or changing the draft-ignore policy. Do not apply that snapshot
over newer draft edits.

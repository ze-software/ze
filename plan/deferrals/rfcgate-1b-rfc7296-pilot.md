# Deferrals -- spec-rfcgate-1b-rfc7296-pilot

Source: `plan/spec-rfcgate-1b-rfc7296-pilot.md`. Format: `ai/rules/deferral-tracking.md`.

Every row below records the same owner decision, taken 2026-07-31: work package WP-9
("Configuration payload and remote access", phase list item 15) was SPLIT. The rows that
need the virtual-address-assignment feature move to `plan/spec-ipsec-remote-access.md`,
which already owns that surface and is named as its owner by
`internal/component/ike/engine/config.go:115`. The rows that are already conformant, and
the two live defects, stay in the pilot.

`ai/rules/deferral-tracking.md` prefers an existing spec that covers the topic over a new
deferral holder, so no `spec-rfcgate-1b-deferred-*` spec was created.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-2.19-2` (the CP payload MUST be inserted before the SA payload). Needs a CP entry between AUTH and SAr2 in the responder's IKE_AUTH response chain | Owner decision 2026-07-31 splitting WP-9. The row needs the address-assignment feature, not a test. Ze builds no CP payload at all today | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-2.19-3` (CP payloads MUST be inserted in the messages containing the SA payloads, when there are multiple IKE_AUTH exchanges). Needs CP handling at all three drop sites, including the post-EAP-Success response | Owner decision 2026-07-31 splitting WP-9. EAP is exactly that variation and Ze implements it, so the row is live rather than hypothetical | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-2.19-5` (the responder MUST NOT send a CFG_REPLY when no CP(CFG_REQUEST) arrived first). Needs the fail-closed request guard that checks payload count, CFG type, attribute presence and profile policy | Owner decision 2026-07-31 splitting WP-9. Vacuously true today because Ze sends no CFG_REPLY. It becomes the feature's primary authorization guard, and `ai/rules/fail-closed-guards.md` governs it | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-2.19-6` (IRAS MUST fail the request and end Child SA creation with FAILED_CP_REQUIRED when its configuration requires CP for an identity and the IRAC sent none). Needs a new YANG policy leaf, a resolved-flag policy lookup, the first use of notify 37, and a short-circuit before the Child SA install | Owner decision 2026-07-31 splitting WP-9. No config leaf expresses the policy, and `NotifyFailedCPRequired` (`internal/component/ike/wire/payload_notify.go:44`) has zero referents | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-3.15.1-1` (only one netmask is allowed in request and response messages, and it MUST be used only with an INTERNAL_IP4_ADDRESS attribute). Needs send-side cardinality and pairing in the CFG_REPLY builder, and receive-side tolerance | Owner decision 2026-07-31 splitting WP-9. `ReadFrom` (`internal/component/ike/wire/payload_cp.go:65`) enforces no cardinality of any kind, and no consumer applies the rule | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-3.15.1-3` (SUPPORTED_ATTRIBUTES MUST be zero-length within a Request). CONDITIONAL on owner item OI-3: the send half binds the requester, which Ze is not, so only the responder-answering half needs code. Attribute type 14 is not a declared constant | Owner decision 2026-07-31 splitting WP-9. Answering the query is strictly more compliance and needs no permission. Declining it is narrower and needs the owner's answer (`ai/rules/rfc-compliance.md`) | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-3.15.1-4` (unrecognized or unsupported attributes MUST be ignored in both requests and responses). Needs the attribute dispatcher, and it is BLOCKED on the RESERVED-bit codec fix that stays in the pilot | Owner decision 2026-07-31 splitting WP-9. Nothing ignores and nothing acts, because there is no consumer. The negative half of its tagged pair IS the reserved-bit case, so the two specs are coupled here | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-4-2` (an implementation supporting these requests MUST parse the CFG_REQUEST CP payload in the first IKE_AUTH message and recognize INTERNAL_IP4_ADDRESS or INTERNAL_IP6_ADDRESS). Must land BEFORE the pilot's `RFC7296-4-4`, see the id-allocation row below | Owner decision 2026-07-31 splitting WP-9. Vacuously conformant today because Ze does not support responding. This spec makes the antecedent true | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-4-3` (an implementation supporting leasing MUST return a CFG_REPLY containing an address of the requested type). Needs type fidelity: an IPv6-only request against an IPv4-only pool gets no address of the wrong family. Must land BEFORE the pilot's `RFC7296-4-4` | Owner decision 2026-07-31 splitting WP-9. Vacuously conformant today, same antecedent as `4-2` | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-1.7-1` (implementations MUST ignore proposals that have configuration attribute type 5, the old value for INTERNAL_ADDRESS_EXPIRY), landing as `RFC7296-1.7-3` because Section 1.7's high-water mark is 2. Implements the semantics corrected by erratum 5056: ignore the ATTRIBUTE, never the proposal. Also settles that `1.7-1` does NOT duplicate `3.15.1-4` (`plan/spec-rfcgate-1b-rfc7296-pilot.md:1289`) | Owner decision 2026-07-31 splitting WP-9. The constant for type 5 is not declared, and the dispatcher that must drop it does not exist | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Honor Section 4 id-allocation order across the two specs. `check_id_allocation` (`scripts/dev/rfc_requirements.py:477-510`) refuses an ordinal at or below its section's high-water mark. Section 4 has THREE claimants and no mark yet: `4-1` (pilot WP-12, item 7), `4-2` and `4-3` (this deferral), `4-4` (pilot WP-10, item 14). Section 4 must land ASCENDING. So the pilot defers `4-4` until `4-2` and `4-3` land, or all four land in one commit | Owner decision 2026-07-31 splitting WP-9. The pilot's own phase list schedules WP-10 (item 14) before WP-9 (item 15), which strands `4-1`, `4-2` and `4-3` permanently. Measured 2026-07-31: no `RFC7296-4-*` id exists at HEAD or in the working tree, so no mark is set yet and the order is still free | `plan/spec-ipsec-remote-access.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Write `test/ipsec/ipsec-child-rekey-no-proposal.ci`, the end-to-end proof that a refused CHILD REKEY draws NO_PROPOSAL_CHOSEN and leaves the IKE SA established. WP-3's design named it | BLOCKED, not chosen. A static config cannot make IKE_AUTH succeed and the later rekey fail. A disjoint `esp-group` fails `selectResponderESP` at IKE_AUTH, so the SA never establishes. And `respondChildRekey` matches against `ESPGroup.Proposals[0]`, which `selectResponderESP` has already narrowed to the negotiated suite. Needs a way to change `esp-group` on a live SA, or a seam that narrows `ESPGroup` after establishment. The handler is covered by `TestErrRefusedChildRekeyIsAnswered`, which drives the real handler and reads the real datagram off a real UDP socket. Detail: `plan/handover/04-wp3-residuals.md` | `plan/spec-rfcgate-1b-rfc7296-pilot.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Write `test/ipsec-interop/scenarios/19-error-notifications/`, the strongSwan proof that a refused CHILD REKEY draws NO_PROPOSAL_CHOSEN and the IKE SA stays up. WP-3's design named it | BLOCKED on the same fact as the row above, against strongSwan instead of a second ze. Also gated by `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md`, because no automated caller runs that tree so the gate refuses a tag placed there | `plan/spec-rfcgate-1b-rfc7296-pilot.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Decide the piggyback of RFC 7296 Section 2.21.2. The responder can send IDr, CERT and AUTH beside the error notify on an IKE_AUTH Child SA failure, and keep the IKE SA alive. WP-3 sends the notify where it once sent silence. It still sets `StateDead` | NOT deferred by choice, and it is a question rather than work. The clause is a MAY, so no gated row is unproven. But it is the one place in WP-3 where Ze knowingly does less than the section offers. `ai/rules/rfc-compliance.md` makes the question mandatory. Cost if the owner wants it: `finishResponderEstablish` (`internal/component/ike/engine/responder.go`) is the only path to `StateEstablished`, and it requires an installed Child SA. An establish path with no Child SA must exist first | `plan/spec-rfcgate-1b-rfc7296-pilot.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-2.22-1` (these payloads MUST NOT occur in messages that do not contain SA payloads). Needs the IPCOMP_SUPPORTED notify, CPI allocation, Child SA binding and both dataplane backends, none of which exist | Owner decision 2026-07-31: create a spec to fully implement IPComp, and do not implement it in this session. RFC 7296 Section 2.22 makes offering and accepting IPComp a MAY. The row is conformant today by non-participation, and the owner chose a real feature over four tests over an absence | `plan/spec-ipsec-ipcomp.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-2.22-2` (an implementation MUST NOT accept an IPComp algorithm that was not proposed). Needs the IPCOMP_SUPPORTED notify, CPI allocation, Child SA binding and both dataplane backends, none of which exist | Owner decision 2026-07-31: create a spec to fully implement IPComp, and do not implement it in this session. RFC 7296 Section 2.22 makes offering and accepting IPComp a MAY. The row is conformant today by non-participation, and the owner chose a real feature over four tests over an absence | `plan/spec-ipsec-ipcomp.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-2.22-3` (an implementation MUST NOT accept more than one IPComp algorithm). Needs the IPCOMP_SUPPORTED notify, CPI allocation, Child SA binding and both dataplane backends, none of which exist | Owner decision 2026-07-31: create a spec to fully implement IPComp, and do not implement it in this session. RFC 7296 Section 2.22 makes offering and accepting IPComp a MAY. The row is conformant today by non-participation, and the owner chose a real feature over four tests over an absence | `plan/spec-ipsec-ipcomp.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Implement and prove `RFC7296-2.22-4` (an implementation MUST NOT compress using an algorithm other than one proposed and accepted in the setup of the Child SA). Needs the IPCOMP_SUPPORTED notify, CPI allocation, Child SA binding and both dataplane backends, none of which exist | Owner decision 2026-07-31: create a spec to fully implement IPComp, and do not implement it in this session. RFC 7296 Section 2.22 makes offering and accepting IPComp a MAY. The row is conformant today by non-participation, and the owner chose a real feature over four tests over an absence | `plan/spec-ipsec-ipcomp.md` | deferred |

## Section 2.19 and Section 3.15.1 ordinals: read this before you allocate an id

The pilot's half of WP-9 landed `RFC7296-2.19-1`, `-2.19-4`, `-2.20-1`, `-3.15.1-2`, `-5`,
`-6` and `-7` at their Appendix A ordinals. The marks were measured first. No
`RFC7296-2.19-*`, `RFC7296-2.20-*` or `RFC7296-3.15.1-*` id existed at HEAD. No mark was set,
and every ordinal was free.

Those rows set the high-water mark to **4 for Section 2.19** and **7 for Section 3.15.1**.
`check_id_allocation` (`scripts/dev/rfc_requirements.py`) refuses a NEW id at or below its
section's mark. Five of the deferred rows above can therefore no longer take their
Appendix A ordinal:

| Deferred row | Appendix A ordinal | Free after the pilot landed? |
|--------------|--------------------|------------------------------|
| `RFC7296-2.19-2` | -2 | **no**, mark is 4. Needs -7 or higher |
| `RFC7296-2.19-3` | -3 | **no**, mark is 4. Needs -7 or higher |
| `RFC7296-2.19-5` | -5 | yes |
| `RFC7296-2.19-6` | -6 | yes |
| `RFC7296-3.15.1-1` | -1 | **no**, mark is 7. Needs -8 or higher |
| `RFC7296-3.15.1-3` | -3 | **no**, mark is 7. Needs -8 or higher |
| `RFC7296-3.15.1-4` | -4 | **no**, mark is 7. Needs -8 or higher |

This is the accepted cost of the package split. It is a renumbering rather than a loss,
because the obligations still land and stay gated. `plan/spec-ipsec-remote-access.md` must
allocate the five ids above their section mark. It must also record the Appendix A ordinal
each one came from, so a reader can still match the row to Appendix A. The same precedent
applies to `RFC7296-1.7-1`. Section 10 of `plan/handover/03-design-wp9.md` renumbers it to
`RFC7296-1.7-3`, because the mark for Section 1.7 is 2.

**Recompute the mark at the moment you land a row. Never hardcode it from this table**:

    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-2\.19-[0-9]*' | sort -V | tail -1

Section 4 is untouched. The pilot's half of WP-9 took no Section 4 ordinal. So `4-1`, `4-2`,
`4-3` and `4-4` are all still free, and the ascending-order rule above still governs them.

## Work package WP-11 left the pilot whole

The four rows above are the entire WP-11 package (phase list item 13). No IPComp row stays
in the pilot. `plan/spec-ipsec-ipcomp.md` carries all four. Section 2.22 has no high-water
mark in `rfc/short/rfc7296.md`, so the four land at `-1` through `-4` in one commit. A
partial landing sets the mark and strands the rest.
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Express UDP encapsulation on the VPP dataplane. `vppBackend.InstallSA` (`internal/component/ike/dataplane/vpp.go`) hardcodes `UDPSrcPort: 0, UDPDstPort: 0` and never reads `p.UDPEncap`, and its hand-rolled `ipsecSAEntry` has no `Flags` field, so `IPSEC_API_SAD_FLAG_UDP_ENCAP` cannot be sent at all | The VPP backend programs NOTHING today: every `InstallSA`, `RemoveSA`, `InstallPolicy` and `RemovePolicy` declares a CRC of `"00000000"` and fails at the first request. The missing encapsulation flag is a symptom of that larger defect, which already has its own spec. WP-8 scoped its work to the Linux/XFRM dataplane Ze ships | `plan/spec-fixit-vpp-ipsec-inoperable.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Land `RFC7296-2.23-6` (reserved ordinal `-2.23-10`) and Appendix A's `RFC7296-2.23-8` (reserved `-2.23-11`): receive and process BOTH ESP forms at any time, and process encapsulated ESP with no NAT detected | MEASURED blocker, not a scope choice. `TestEncapKernelBindsOneESPFormPerState` shows a Linux XFRM inbound state accepts bare ESP or UDP-encapsulated ESP, never both, and two states on one SPI do not help. Full compliance is on the table, so `ai/rules/rfc-compliance.md` reserves the decision for Thomas. Raised as OR-WP8-4. No `{gap}` and no `partial` was written. Writing one IS the decision | `plan/spec-rfcgate-1b-rfc7296-pilot.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Meet the source-ADDRESS half of `RFC7296-2.11-3` on a multi-homed host. Both IKE listeners bind the wildcard by default. `WriteToUDP` therefore lets the route table choose the source address. `pkt.LocalAddr` is captured and no sender reads it | WP-8 implemented and tagged the PORT half only. Its tag says so. The address half needs `IP_PKTINFO` plus a `sendmsg` path, or one socket per local address. Both change listener lifecycle. The choice is the owner's (OR-WP8-1) | `plan/spec-rfcgate-1b-rfc7296-pilot.md` | deferred |
| 2026-07-31 | spec-rfcgate-1b-rfc7296-pilot | Fix defect 2 of `plan/spec-fixit-ike-responder-natt-port-float.md`: `handleResponderEAP` sets `StateDead` on a retransmitted IKE_AUTH instead of replaying the cached response (RFC 7296 Section 2.1) | WP-8 subsumed defect 1 of that spec and NARROWED it rather than closing it. Defect 2 touches none of WP-8's producers and has no row among the pilot's 113, so absorbing it would have shipped an untagged protocol fix (OR-WP8-3) | `plan/spec-fixit-ike-responder-natt-port-float.md` | deferred |

## Two items the design documents name, and neither is deferred

`plan/handover/03-design-wp10.md` (R-WP10-16) and `plan/handover/03-design-wp7.md` (R-WP7-6)
each discuss whether a finding is in scope. Both are recorded here as IN SCOPE, so no reader
mistakes the discussion for a deferral.

| Item | Where it belongs | Why it is not deferred |
|------|------------------|------------------------|
| The `dh-group` leaf accepts groups 1 to 31 with no implementability gate, while only 14, 19 and 20 exist. It surfaced while `RFC7296-3.3.4-1` was established | WP-10, inside this pilot | `ai/rules/no-parking.md`: a defect you find while you do something else is the reason you are the one who fixes it. It is a one-predicate change that mirrors the existing gate in `ipsec/config.go` |
| Traffic selectors have no config surface, so there is nothing for `RFC7296-2.9-2` to narrow against | WP-7, inside this pilot | The policy IS the antecedent of the row. Narrowing written against a hardcoded policy is dead on arrival, and `narrowTS` in its current state is the proof of what that looks like |


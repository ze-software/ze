# Spec: bgp-unowned-rfc-rows -- the BGP RFC rows no spec owned

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The BGP child of the rfc-verdict pass (`plan/pre-release/spec-rfc-verdict-fix-bgp.md`,
"Gap accounting", around line 975) closed with eleven requirement rows that are
canonically `unimplemented` and that no spec owned. Owner decision, 2026-10-11:
"10 give it a spec". This spec owns them.

One of the eleven already had an owner the child did not name: `RFC9252-5-2` (SRv6
Service SID resolvability before best path) is the subject of
`plan/immediate/spec-srv6-bestpath-resolvability.md` (its lines 27, 63 and 453-454
carry the row and its tags). It stays there, and the child spec now points at it.
The ten rows below are this spec's.

| # | Row id | Document, section | Class | What Ze does today |
|---|--------|-------------------|-------|--------------------|
| 1 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4` | draft-ietf-bess-mup-safi §3.3.6 | defect | Stores a received Direct Segment Discovery (DSD) route without comparing its MP_REACH next hop to the originator of the Prefix-SID locator |
| 2 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5` | draft-ietf-bess-mup-safi §3.3.6 | defect | Stores a received DSD route that carries no Prefix-SID attribute |
| 3 | `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2` | draft-ietf-idr-linklocal-capability §1 | defect | Accepts a Link-Local next hop on a capability-77 session and publishes it to the RIB, sysrib and FIB as a bare `fe80::/10` address with no interface; the sent `session link-local` address is not tied to the session's interface |
| 4 | `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1` | draft-ietf-idr-linklocal-capability §6 | defect | On a capability-77 session, admits a 32-octet Next Hop field without checking that it is a Global followed by a Link-Local |
| 5 | `RFC2545-3-6` | RFC 2545 §3 | defect (fix needs a source Ze lacks) | Sends a third-party global next hop on a common subnet in 16 octets, without the Link-Local of that next hop |
| 6 | `RFC4271-8.2.2-13` | RFC 4271 §8.2.2 (Active) | gap: OPTIONAL feature absent | No DelayOpen, so Active never holds a connection and the Event 18 action list is reached by no layer |
| 7 | `RFC4271-8.2.2-20` | RFC 4271 §8.2.2 (Idle) | gap: optional event absent | No Event 3; automatic restart is Event 6, which keeps the ConnectRetryCounter |
| 8 | `RFC9086-7-2` | RFC 9086 §7 | gap: feature absent | Only PeerNode SID configuration exists; no PeerAdj or PeerSet SID, no per-recipient control |
| 9 | `RFC9830-2.4.2-12` | RFC 9830 §2.4.2 | gap: feature absent | Never signals an SRv6 BSID in the type-13 Binding SID sub-TLV (uses the RECOMMENDED type 20) and decodes no type 13 on receipt |
| 10 | `RFC7606-5.1-1` | RFC 7606 §5.1 | defect | Emits MP_REACH_NLRI at its type-code position, after ORIGIN and AS_PATH, in all three UPDATE builders |

The class column follows `ai/rules/rfc-compliance.md`: "Behavior that violates an
applicable RFC requirement in a capability Ze implements is a defect; a capability Ze
does not provide is an implementation gap". Defects are fixed in this spec. The four
gap rows do not authorize implementation on their own: each is an Open Question for
the owner below, and its AC applies only if he scopes the feature in. If he scopes
it out, the AC becomes a ledger correction (exclusion or owner-approved deviation,
recorded with its RFC section and reason, never counted conformant).

### Bucket

`plan/immediate/`. Six of the ten rows are wire or routing correctness defects an
operator on the first release meets: a MUP DSD route that should be withdrawn is
installed (rows 1, 2), a Link-Local route is installed with no outgoing interface or
a malformed next hop is accepted (rows 3, 4), a third-party next hop loses its
Link-Local (row 5), and every MP UPDATE Ze sends violates an RFC 7606 SHALL that
FRR and BIRD both meet (row 10). `plan/README.md` puts wire correctness and routing
correctness in `immediate/`. The four gap rows on their own would sit in `plan/`;
they ride here because the owner asked for one owning spec. If he scopes them out,
nothing in this spec waits on them; if he scopes them in and wants them split to
`plan/`, that is a relocation the owner makes.

### The RFC text, quoted from the source

| # | Quote | Read at |
|---|-------|---------|
| 1 | "However, the receiving BGP speaker MUST ensure that the received nexthop value in the MP_REACH_NLRI attribute is identical to the originator of the locator value in the prefix SID attribute. The originator of the locator value can be resolved from the IPv6 IGP table. If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied." | `rfc/drafts/draft-ietf-bess-mup-safi.txt` §3.3.6 |
| 2 | "When a BGP speaker receives a MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied." | same, §3.3.6 |
| 3 | "Since IPv6 Link-Local addresses are not required to be globally unique, implementations must ensure that they are strictly associated with a specific interface." | `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` §1 |
| 4 | "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of "treat-as-withdraw", as described in section 7.3 of [RFC7606]." Section 6 opens: "These procedures apply only when the Link-Local Next Hop Capability has been negotiated for a BGP session." | same, §6 |
| 5 | "The link-local address shall be included in the Next Hop field if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to." | `rfc/full/rfc2545.txt` §3 |
| 6 | "If the local system receives a TcpConnectionFails event (Event 18), the local system: - restarts the ConnectRetryTimer (with the initial value), - stops and clears the DelayOpenTimer (sets the value to zero), - releases all BGP resource, - increments the ConnectRetryCounter by 1," and §8.2.1.3: "If the flag indicating support for an optional timer (DelayOpen or DampPeerOscillations) cannot be set to TRUE, the timers and events supporting that option do not have to be supported." | `rfc/full/rfc4271.txt` §8.2.2 Active, §8.2.1.3 |
| 7 | "In response to a ManualStart event (Event 1) or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero," and §8.1.2 Event 3 "Status: Optional, depending on local system" | `rfc/full/rfc4271.txt` §8.2.2 Idle, §8.1.2 |
| 8 | "The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID, PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer." | `rfc/full/rfc9086.txt` §7 |
| 9 | "The value MUST be 18 when a SRv6 BSID is present, 6 when an SR-MPLS BSID is present, or 2 when no BSID is present." Same section: "It is RECOMMENDED that the SRv6 Binding SID sub-TLV, as defined in Section 2.4.3, be used when signaling an SRv6 BSID for an SR Policy CP. The support for the use of this Binding SID sub-TLV for the signaling of an SRv6 BSID is retained primarily for backward compatibility" | `rfc/full/rfc9830.txt` §2.4.2 |
| 10 | "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if present) SHALL be encoded as the very first path attribute in an UPDATE message." | `rfc/full/rfc7606.txt` §5.1 |

### How FRR and BIRD handle it

| # | FRR (`/Users/thomas/Code/github.com/FRRouting/frr`) | BIRD (`/Users/thomas/Code/gitlab.nic.cz/labs/bird`) |
|---|------|------|
| 1, 2 | No MUP SAFI in `bgpd/` | No MUP SAFI in `proto/bgp/` |
| 3 | `bgpd/bgp_attr.c` (MP_REACH next hop parse, around line 2644): a Link-Local global next hop takes `peer->nexthop.ifp->ifindex` as `nh_ifindex`; with no peer interface the route is withdrawn (`BGP_ATTR_PARSE_WITHDRAW`) | `proto/bgp/packets.c` around line 1113: the Link-Local is resolved with `neigh_find(..., p->neigh->iface, ...)`, the session's interface; not directly reachable is a withdraw |
| 4 | Same function, around line 2687: a 32-octet field whose second address is not Link-Local is logged at debug and the Link-Local half dropped | `proto/bgp/packets.c` around line 1432: a second address that is not Link-Local is dropped and the field treated as 16 octets |
| 5 | Not compared (out of budget for this pass; see A-5) | `bgp_update_next_hop_ip` (packets.c around line 1280): a third-party gateway is sent in 16 octets, no Link-Local. BIRD has the same omission |
| 6 | DelayOpen implemented (`bgpd/bgp_fsm.c`, `bgp_delayopen_timer`) | No DelayOpen; restart pacing is `error_delay_time_min/max` backoff (`proto/bgp/bgp.c` around line 1018), the same shape as Ze's Event 6 |
| 7 | No RFC ConnectRetryCounter tracked; FRR counts `peer->dropped` | No RFC ConnectRetryCounter |
| 8 | No PeerAdj or PeerSet SID in `bgpd/` | None |
| 9 | SR Policy lives in `pathd`, not compared | None |
| 10 | `bgp_packet_attribute` (`bgpd/bgp_attr.c` line 4950) writes MP_REACH (`bgp_packet_mpattr_start`, line 4976) before ORIGIN (line 4988) | `bgp_create_mp_reach` (packets.c line 2895) writes MP_REACH first, then copies the other attributes after it |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/mp-nlri-ordering.md` - the page that states Ze's MP attribute order
  → Constraint: this page states the current order (MP_UNREACH first, everything else by type code, MP_REACH included) and argues it; row 10's fix makes the page wrong, so the page edit lands in the same change as the builder edit
  → Decision: the page's "three builders disagreed" history is the reason the fix goes into `attribute.OrderAttributes` and every builder at once, never one builder
- [ ] `docs/architecture/behavior/fsm.md` - names DelayOpen and the timers Ze does not implement
  → Constraint: rows 6 and 7 change this page only if the owner scopes DelayOpen or Event 3 in
- [ ] `docs/architecture/testing/interop.md` - scenario naming and the revert-and-red walk
  → Constraint: every new scenario is named (no numeric prefix) and owes a recorded red with the fix reverted and the artifact rebuilt

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/draft-ietf-bess-mup-safi.md` - rows 3.3.6-4 (line 422), 3.3.6-5 (line 423)
  → Constraint: both `{gap}` annotations name `internal/component/bgp/plugins/nlri/mup/rfc7606.go::RecognizeNLRI` as reading the architecture and route type alone
- [ ] `rfc/short/draft-ietf-idr-linklocal-capability.md` - rows 1-2 (line 109), 6-1 (line 129) and the `Support coverage` gap sentence
  → Constraint: 6-1's annotation separates a wrong Next Hop length (session reset under RFC 7606 §7.11, already met by `validateMPReachNextHop`) from a wrong address shape inside a 32-octet field (this row); the fix must not turn the first into the second
- [ ] `rfc/short/rfc2545.md` - row 3-6 (line 207) and the `Support remaining` partial text
  → Constraint: the gap covers only rails where Ze writes the next hop; a received complete 32-octet pair relayed unchanged is not in it (main-thread ruling R53(b), `plan/handover/rfc-verdict-test-fix-pass/RULINGS.md`)
- [ ] `rfc/short/rfc4271.md` - rows 8.2.2-13 (line 811), 8.2.2-20 (line 805)
  → Constraint: main-thread rulings R47(a) and R53(a) hold 8.2.2-20 as a counted gap because Ze has an automatic start, and 8.2.2-13 as a `{gap}` stamped unimplemented because the gate refused `{feature-declined}` beside existing tags. Any exclusion the owner chooses must be expressible in today's ledger, or the gate change is part of this spec
- [ ] `rfc/short/rfc9086.md` - rows 7-1 (partial, line 372) and 7-2 (line 373)
  → Constraint: 7-2 is the R3 split of 7-1's sentence; resolving 7-2 also settles 7-1's gap half
- [ ] `rfc/short/rfc9830.md` - row 2.4.2-12 (line 312)
- [ ] `rfc/short/rfc7606.md` - row 5.1-1 (line 338), and the `docs/features/rfc-status.md` RFC 7606 row that discloses it publicly
  → Constraint: the public disclosure is a published claim; when the fix lands, the row and the generated status page change in the same commit

**Key insights:**
- Ten rows, six defects, four absent features. RFC9252-5-2 is owned elsewhere.
- Row 10 touches every MP UPDATE Ze sends; it is the widest change and the only one with no design freedom.
- Rows 3 and 4 share the receive next-hop path; rows 3 and 5 share the "Link-Local needs an interface or a neighbor" question.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/nlri/mup/rfc7606.go` - `RecognizeNLRI` (line 53) and `MUPRouteType.Implemented` (line 31) decide per-NLRI recognition from architecture and route type only; DSD is an implemented type
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `mpNextHopAddr` (line 1491) takes the first 16 octets of a 32-octet field and parses it with no zone; a 16-octet Link-Local becomes a bare address. `isSRv6Ineligible` (line 1318) reads Prefix-SID type 40 from the stored other-attributes
- [ ] `internal/component/sysrib/sysrib.go` - `nextHopInterface` (line 149) is filled only from a producer that names one; the BGP best-change event names none
- [ ] `internal/plugins/fib/kernel/mplsentry_linux.go` - `resolveMPLSNextHop` (line 152) refuses a Link-Local next hop with no zone ("link-local next hop requires an interface zone"); the MPLS path already knows the need
- [ ] `internal/component/bgp/fsm/fsm.go`, `internal/component/bgp/fsm/state.go` - Event 6 implemented as the reactor's exponential backoff (state.go line 162); Events 3 and DelayOpen absent by recorded design (fsm.go header lines 16-18, 77)
- [ ] `internal/component/bgp/reactor/session.go` - line 1006-1011 fires Event 6 for an automatic restart
- [ ] `internal/core/bgp/attribute/origin.go` - `OrderAttributes` (line 220) and `WriteAttributesOrdered` (line 269); the comment above line 220 records why MP_REACH moved to its type-code position and names the other two builders (`reactor/peer_rib_routes.go::buildRIBRouteUpdate`, `reactor/reactor_api_batch.go::buildWireModeUpdate`)
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/config.go` - `buildBindingSIDSubTLV` / `buildBindingSIDNullSubTLV` emit type 13 with 6 or 2 octets only; no file in `internal/component/bgp` decodes type 13
- [ ] `internal/component/bgp/plugins/epe/epe_config.go::ParseConfig`, `internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig` - PeerNode only, no per-recipient control (per the rfc/short annotation; re-read at implementation)
- [ ] `internal/component/bgp/reactor/link_scope.go` - `linkLocalNextHop`, `sameLinkLayerSegment`; the send-side Link-Local selection rows 3 and 5 extend

**Behavior to preserve:**
- A wrong Next Hop LENGTH stays a session reset (`validateMPReachNextHop`, RFC 7606 §7.11).
- MP_UNREACH_NLRI-only UPDATEs keep their current byte layout apart from position rules row 10 imposes.
- Event 6 backoff semantics and `TestRFC4271ConnectRetryCounterSurvivesDampedStart`.
- PeerNode SID configuration and its tests (RFC9086-7-1 tested half).
- A received complete Global + Link-Local pair relayed unchanged.

**Behavior to change:** the ten rows, as the ACs state, with rows 6-9 conditional on the owner's answers.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Receive: UPDATE wire bytes on a peer session (rows 1-4).
- Send: route origination, reflection or relay through the three UPDATE builders (rows 5, 9, 10).
- FSM: the peer run loop's automatic restart and TCP events (rows 6, 7).
- Config: EPE and SR Policy config (rows 8, 9, if scoped in).

### Transformation Path
1. Wire parse and RFC 7606 classification in `internal/component/bgp/message/` (per-NLRI recognition calls into the family plugin's `RecognizeNLRI`).
2. Admission to the RIB plugin, best path in `rib_bestchange.go`, best-change event to sysrib.
3. sysrib selection, FIB programming in `internal/plugins/fib/kernel/` or `fib/vpp/`.
4. Egress: `reactor/link_scope.go` next-hop scope, attribute assembly, `attribute.OrderAttributes` / `WriteAttributesOrdered`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to RIB plugin | received UPDATE with per-NLRI treat-as-withdraw already decided | No |
| RIB plugin to sysrib | best-change event; today carries no interface for a next hop | No |
| sysrib to FIB | route with `nextHopInterface` | No |

### Integration Points
- `RecognizeNLRI` per family (MUP) - row 1 and 2 checks need the UPDATE's Prefix-SID attribute, which the recognizer does not see today (A-1).
- Session interface knowledge in the reactor (`link_scope.go`) - rows 3 and 5.
- `OrderAttributes` - row 10.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be shown at implementation |
| No unintended coupling (components stay isolated) | No | row 1's IGP lookup must go through sysrib, never import an IGP plugin |
| No duplicated functionality (extends existing, does not recreate) | No | row 10 changes the one ordering function and the two builders that reimplement it |
| Zero-copy preserved where applicable (refs, not copies) | No | row 10 reorders in place in the build buffer; no extra copy |
| Registration over hardcoding, outbound | No | MUP checks live in the MUP plugin's registered recognizer, not in the engine |
| Registration over hardcoding, inbound | No | to be shown at implementation |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The MUP per-NLRI recognizer can be given the UPDATE's Prefix-SID attribute without a central switch | `RecognizeNLRI` signature takes NLRI bytes only | Rows 1-2 need an engine-side hook that learns about MUP, which `ai/rules/principles.md` refuses; a registered per-family attribute check is the alternative | Read the engine call site of `RecognizeNLRI` | unvalidated |
| A-2 | The session's interface is known in the reactor for a directly connected IPv6 peer | `link_scope.go::sameLinkLayerSegment` answers segment questions from connected prefixes | Row 3 needs an interface source; without it the route must be withdrawn, the FRR behavior | `gopls` on `link_scope.go`, read the connected-prefix source | unvalidated |
| A-3 | Ze never emits MP_REACH and MP_UNREACH in one UPDATE | RFC 7606 §5.1 second bullet forbids it | If it does, row 10's "very first" has two candidates and a second RFC 7606 §5.1 defect exists, which gets a journal row | grep the builders, a unit test over a mixed batch | unvalidated |
| A-4 | GoBGP implements the MUP SAFI and can send a DSD route with and without a Prefix-SID | GoBGP release notes (unchecked here) | Rows 1-2 interop falls back to a ze-peer raw-hex functional test, and the interop gap is disclosed | Build the GoBGP container, send a DSD route | unvalidated |
| A-5 | FRR's behavior for row 5 was not read in this pass | budget | The FRR comparison for row 5 is missing from the design | Read `bgpd/bgp_updgrp_packet.c` next-hop rewrite before design approval | unvalidated |
| A-6 | About 118 `.ci` files carry MP attribute hex (`800e`/`900e`) | `grep -rli` over `test/` | Row 10's expectation rewrite is larger or smaller than planned | rerun the count at implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Row 10 changes every MP UPDATE byte string: exact-hex `.ci`, golden files and interop captures go red together | the first `./le test functional plugin` run after the change | Regenerate expectations mechanically from the new builder, review a sample by hand, and land code and expectations in one commit |
| R-2 | Row 10 desynchronises the three builders again | two builders emitting different bytes for one route | One test feeds the same route through all three builders and compares bytes (exists for type-code order; extend it) |
| R-3 | Row 1 needs an IPv6 IGP lookup; with no IGP route for the locator Ze would withdraw every DSD route | DSD routes vanish in a lab without IS-IS | Open Question OQ-1 decides the behavior when the originator cannot be resolved |
| R-4 | Row 3 withdraws Link-Local routes Ze used to install (badly) | operators see fewer routes after upgrade | Log the withdraw at Warn with the reason and the fix; the route was not forwardable before |
| R-5 | Row 4 withdraws routes from a peer sending two Link-Locals, a form the draft says is "observed in deployment" (BIRD) | interop with BIRD loses routes | The draft says receivers SHOULD use the second address for that form, so it is not malformed; the AC excludes it explicitly |
| R-6 | A ledger exclusion for rows 6-9 is refused by `./le rfc check` (the R50-R53 history) | gate exit 2 | The exclusion form is designed with the gate, or the row stays a counted gap with the owner's decision recorded |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Row 10: every MP UPDATE Ze sends. Rows 1-4: routes withdrawn that should have been kept. Row 7: restart pacing |
| How is it reverted? | Each row lands as its own commit, revertible alone |
| Who else touches this path? | `plan/immediate/spec-srv6-bestpath-resolvability.md` (same RIB candidate filter as row 1's Prefix-SID read), `plan/immediate/spec-bgp-prefix-sid-rfc-defects.md` (Prefix-SID parsing), `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (ledger rows) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE with DSD route, no Prefix-SID, from a peer | → | MUP recognizer treat-as-withdraw | `test/plugin/mup-dsd-without-prefix-sid-withdraw.ci` |
| UPDATE with DSD route whose next hop differs from the locator originator | → | MUP recognizer, sysrib lookup | `test/plugin/mup-dsd-nexthop-locator-mismatch-withdraw.ci` |
| UPDATE with 16-octet Link-Local next hop on a capability-77 session | → | RIB best change carries the session interface to sysrib and FIB | `test/plugin/linklocal-capability-nexthop-interface.ci` |
| UPDATE with a malformed 32-octet next hop on a capability-77 session | → | treat-as-withdraw | `test/plugin/linklocal-capability-malformed-nexthop-withdraw.ci` |
| Static route with a third-party global next hop on the common subnet | → | `link_scope.go` third-party Link-Local | `test/plugin/rfc2545-third-party-linklocal.ci` |
| Any MP announcement from config | → | `OrderAttributes` and both builders | `test/plugin/rfc7606-mp-reach-first.ci` |
| Rows 6-9 | → | only if scoped in (OQ-3 to OQ-6) | named when the owner answers; a scoped-out row has no wiring row and its ledger correction is the deliverable |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (row 1) | A DSD route arrives whose MP_REACH next hop is not the originator of the Prefix-SID locator, as the IPv6 IGP table resolves it | The NLRI is treated as withdrawn, the rest of the UPDATE is processed, and a Warn log names the peer, the route, the next hop and the resolved originator. Behavior when the originator cannot be resolved follows OQ-1. A matching next hop is stored (positive polarity) |
| AC-2 (row 2) | A DSD route arrives in an UPDATE with no Prefix-SID attribute | The NLRI is treated as withdrawn and the rest of the UPDATE is processed; the same route with a Prefix-SID is stored |
| AC-3 (row 3) | A Link-Local next hop (16 octets, or the Link-Local half of 32) arrives on a session over a known interface | The installed kernel route names that interface as its device; on a session with no known interface the route is treated as withdrawn with a Warn log naming the reason (FRR behavior). On send, the advertised own Link-Local is an address of the session's interface, and a configured `session link-local` that is not on that interface is refused at commit with an error naming the leaf, the interface and the fix |
| AC-4 (row 4) | On a capability-77 session, a 32-octet Next Hop field that is not a Global (or `::`) followed by a Link-Local, and not the two-Link-Local form §6 tells receivers to accept | The UPDATE is handled as treat-as-withdraw (RFC 7606 §7.3); a well-formed pair and the two-Link-Local form are accepted; a wrong length still resets the session |
| AC-5 (row 5) | Ze advertises a route whose global next hop is another router on the subnet it shares with the recipient | Per OQ-2: either the 32-octet field carries that router's Link-Local, or the deviation is recorded in `rfc/short/rfc2545.md` with its section and reason and is not counted conformant |
| AC-6 (row 6) | Per OQ-3 | Scoped in: DelayOpen is configurable, and Event 18 in Active performs the four quoted actions. Scoped out: the row's ledger form is settled so `./le rfc check` accepts it, and `rfc/short/rfc4271.md` Support text names DelayOpen as an implementation gap |
| AC-7 (row 7) | Per OQ-4 | Scoped in: an automatic start that is not damped fires Event 3 and zeroes the ConnectRetryCounter, while a damped restart keeps Event 6 behavior. Scoped out: recorded as the owner decides, not counted conformant |
| AC-8 (row 8) | Per OQ-5 | Scoped in: PeerAdj and PeerSet SIDs are configurable, can be enabled and disabled, and the advertised EPE information is controllable per recipient peer. Scoped out: recorded, and RFC9086-7-1's gap half follows the same decision |
| AC-9 (row 9) | Per OQ-6 | Scoped in: a type-13 sub-TLV carrying an SRv6 BSID is 18 octets on send, and on receipt a type-13 length other than 18, 6 or 2 is handled per RFC 9830 §5. Scoped out: excluded as the backward-compatibility form RFC 9830 recommends against, with the quote |
| AC-10 (row 10) | Ze sends any UPDATE carrying MP_REACH_NLRI or MP_UNREACH_NLRI, from any of the three builders | That attribute is the first path attribute on the wire; the same route through the three builders yields identical bytes; the `.ci` expectations, `docs/architecture/wire/mp-nlri-ordering.md` and the public RFC 7606 status row change in the same commit |
| AC-11 (ledger) | Each row once its AC is met | The row in `rfc/short/` loses `{gap}` or takes the owner's recorded form, carries tags in both polarities where Ze can observe the behavior, and carries a discrimination record (`./le rfc discriminate-record`); `./le rfc check` exits clean for these stems |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with a MUP speaker that sends a malformed DSD route | wire -> MUP recognizer -> treat-as-withdraw -> no RIB entry | `test/plugin/mup-dsd-without-prefix-sid-withdraw.ci` |
| 2 | Peers over an interface with capability 77 and receives Link-Local-only next hops | wire -> RIB -> sysrib -> kernel route with device | `bgp-linklocal-capability-interface-frr` |
| 3 | Sends IPv6 routes to FRR | config -> builder -> wire with MP_REACH first | `bgp-mp-reach-first-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMUPDSDWithoutPrefixSIDTreatAsWithdraw` | `internal/component/bgp/plugins/nlri/mup/rfc7606_test.go` | AC-2 both polarities | |
| `TestMUPDSDNextHopLocatorMismatch` | same | AC-1 both polarities, and the OQ-1 unresolvable case | |
| `TestLinkLocalNextHopCarriesSessionInterface` | `internal/component/bgp/plugins/rib/` | AC-3 receive | |
| `TestLinkLocalSessionAddressMustBeOnInterface` | `internal/component/bgp/plugins/llnh/` | AC-3 send-side refusal | |
| `TestLinkLocalCapabilityMalformedThirtyTwoOctet` | `internal/component/bgp/message/` | AC-4, including the two-Link-Local accepted form and the wrong-length reset | |
| `TestRFC2545ThirdPartyLinkLocal` | `internal/component/bgp/reactor/link_scope_test.go` | AC-5 if scoped in | |
| `TestOrderAttributesMPFirst` | `internal/core/bgp/attribute/origin_test.go` | AC-10 | |
| `TestThreeBuildersSameBytesMPFirst` | `internal/component/bgp/reactor/` | AC-10, R-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Next Hop length on a capability-77 session | 16 or 32 | 32 | 15 (session reset, unchanged) | 33 (session reset, unchanged) |
| Binding SID sub-TLV length (if OQ-6 scopes in) | 2, 6, 18 | 18 | 1 | 19 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mup-dsd-without-prefix-sid-withdraw` | `test/plugin/` | AC-2 | |
| `mup-dsd-nexthop-locator-mismatch-withdraw` | `test/plugin/` | AC-1 | |
| `linklocal-capability-nexthop-interface` | `test/plugin/` | AC-3 | |
| `linklocal-capability-malformed-nexthop-withdraw` | `test/plugin/` | AC-4 | |
| `rfc2545-third-party-linklocal` | `test/plugin/` | AC-5 | |
| `rfc7606-mp-reach-first` | `test/plugin/` | AC-10 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-mp-reach-first-frr` | `test/interop/scenarios/` | FRR | FRR receives Ze's IPv6 UPDATE with MP_REACH as the first attribute (capture asserted), routes accepted | |
| `bgp-mp-reach-first-bird` | same | BIRD | the same against BIRD | |
| `bgp-linklocal-capability-interface-frr` | same | FRR (capability 77, `PEER_HAS_LINK_LOCAL_CAPABILITY`) | FRR sends Link-Local-only next hops; Ze's kernel route names the session interface | |
| `bgp-mup-dsd-treat-as-withdraw-gobgp` | same | GoBGP (A-4) | GoBGP sends DSD routes with and without Prefix-SID; Ze withdraws the second | |
| `bgp-rfc2545-linklocal-nexthop-frr` (existing, extended) | same | FRR | third-party next hop carries its Link-Local, if OQ-2 scopes it in | |
| `bgp-delayopen-frr` | same | FRR | only if OQ-3 scopes DelayOpen in | |

Row 4's malformed form cannot be produced by a conforming peer; its proof is the ze-peer functional test. Row 7 is not wire-visible. Rows 8 and 9 name their scenario when the owner scopes them in. Every scenario owes the revert, rebuild, red, restore, green walk with the red recorded.

## Files to Modify
- `internal/component/bgp/plugins/nlri/mup/rfc7606.go` - rows 1, 2
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - row 3 (next hop carries an interface)
- `internal/component/sysrib/sysrib.go` - row 3 (accept the interface from the BGP producer)
- `internal/component/bgp/message/rfc7606.go` - row 4
- `internal/component/bgp/plugins/llnh/llnh.go` - row 3 send-side refusal
- `internal/component/bgp/reactor/link_scope.go` - rows 3, 5
- `internal/core/bgp/attribute/origin.go`, `internal/component/bgp/reactor/peer_rib_routes.go`, `internal/component/bgp/reactor/reactor_api_batch.go` - row 10
- `internal/component/bgp/fsm/fsm.go`, `internal/component/bgp/fsm/state.go`, `internal/component/bgp/reactor/session.go` - rows 6, 7 if scoped in
- `internal/component/bgp/plugins/epe/epe_config.go`, `internal/component/bgp/plugins/ls_export/export_config.go` - row 8 if scoped in
- `internal/component/bgp/plugins/nlri/srpolicy/config.go` - row 9 if scoped in
- `docs/architecture/wire/mp-nlri-ordering.md` - row 10
- `rfc/short/draft-ietf-bess-mup-safi.md`, `rfc/short/draft-ietf-idr-linklocal-capability.md`, `rfc/short/rfc2545.md`, `rfc/short/rfc4271.md`, `rfc/short/rfc9086.md`, `rfc/short/rfc9830.md`, `rfc/short/rfc7606.md` - AC-11
- `test/plugin/*.ci` carrying MP attribute hex - row 10 expectations

## Files to Create
- the six `test/plugin/*.ci` files and the interop scenarios named above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Only if OQ-3 (DelayOpen timer), OQ-5 (PeerAdj/PeerSet) or OQ-6 scope a feature in; those name the YANG module then |
| YANG validation constraints | No | as above |
| YANG custom validators | Yes | row 3 send side: `session link-local` must be on the session's interface, through the existing `refuseLinkLocalCapabilityWithoutAddress` path in `llnh.go` |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no new leaf unless scoped in |
| Functional test for new RPC/API | N-A | no new RPC; behavior `.ci` tests are listed above |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new runtime dependency; row 3 reads interface state Ze already reads |
| Prometheus counters/metrics | No | treat-as-withdraw counting already exists in the RFC 7606 path; confirm at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family, capability or attribute; MUP and capability 77 exist |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/bgp-protocol.md` if its evidence names these rows (check `./le feature report`) |
| 2 | Config syntax changed? | No | unless OQ-3/5/6 scope a feature in |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` only if the MUP or llnh plugin's described behavior changes |
| 6 | Has a user guide page? | Yes | the IPv6 next-hop guide page that covers `session link-local`, found through `ai/INDEX.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/mp-nlri-ordering.md` |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the seven `rfc/short/` files above and `docs/features/rfc-status.md` (generated by `./le rfc index-update`) |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists DelayOpen or MP attribute order |
| 12 | Internal architecture changed? | Yes | `docs/architecture/behavior/fsm.md` if rows 6-7 scope in |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by the files this spec changes (`// Design:` headers): `docs/architecture/wire/messages.md` and `docs/architecture/wire/attributes.md` (row 10 attribute order: affected, update with the builder change); `docs/architecture/plugin/rib-storage-design.md` (row 3 next hop with interface: affected); `docs/architecture/wire/nlri-bgpls.md` (row 8: affected only if OQ-5 scopes it in); `docs/architecture/config/syntax.md` (row 3 send-side refusal: affected only if the refusal text appears there, else unaffected); `docs/architecture/core-design.md` (unaffected: no change to the core/registration design). Re-derive at implementation with `./le spec citation anchors spec plan/immediate/spec-bgp-unowned-rfc-rows.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | check `session link-local` examples against the new refusal |

## Implementation Steps

1. **Phase: Wiring** -- the six wiring `.ci` tests written and red.
2. **Phase: row 10** -- `OrderAttributes` and the two builders put MP_REACH and MP_UNREACH first; expectations regenerated; page and RFC 7606 row edited. Own commit.
3. **Phase: rows 2 then 1** -- MUP recognizer reads the Prefix-SID presence (A-1), then the locator originator (after OQ-1). Own commit.
4. **Phase: rows 3 and 4** -- receive next hop carries the session interface through sysrib to the FIB; malformed 32-octet check; send-side refusal. Own commit.
5. **Phase: row 5** -- per OQ-2.
6. **Phase: rows 6-9** -- per OQ-3 to OQ-6: implementation, or the ledger correction.
7. **Phase: ledger** -- AC-11 for every row, discrimination records, `./le rfc check` clean for the stems.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation or an owner-recorded ledger form at file:line |
| Correctness | Row 4 does not reclassify a wrong length; the two-Link-Local form is accepted |
| Correctness | Row 10 holds for every builder, and for MP_UNREACH-only UPDATEs |
| Data flow | MUP checks stay in the MUP plugin; no MUP name in the engine |
| Rule: rfc-compliance | Each new implementing function quotes its RFC sentence; each caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `{gap}` left on a scoped-in row | `grep -n` each id in its `rfc/short/` file |
| Both polarities tagged | `grep -rn "RFC requirement: <id>" internal/ test/` |
| Ledger clean | `./le rfc check` exit status for the seven stems |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Rows 1, 2, 4 parse peer-supplied bytes; bounds checked before reading the Prefix-SID or the second address |
| Resource exhaustion | Row 1's IGP lookup per DSD route must not block the receive path |
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

## Open Questions (owner)

| ID | Question | Impact |
|----|----------|--------|
| OQ-1 | Row 1: when the locator's originator cannot be resolved from the IPv6 IGP table (no IGP, or no route), is the DSD route treated as withdrawn or kept? The draft says the originator "can be resolved" and gives no rule for failure | Withdraw is the strict reading and drops every DSD route in a lab without IS-IS; keep leaves a hole the check exists to close |
| OQ-2 | Row 5: fix it by looking up the third-party router's Link-Local in the kernel neighbor table, or record an owner-approved deviation? BIRD has the same omission | Fix: a netlink neighbor read on the send path (Linux only; a doctor check may follow). Deviation: the row stays uncounted with its reason |
| OQ-3 | Row 6: scope DelayOpen in, or decline it? It is OPTIONAL (§8.2.1.3). FRR has it, BIRD does not. Declining needs a ledger form `./le rfc check` accepts (rulings R50-R53 found none today) | In: an FSM timer, a config leaf, an FRR interop scenario. Out: a gate or ledger change, or the row stays a counted gap |
| OQ-4 | Row 7: add Event 3 (an automatic start without damping that zeroes the counter), or record that Ze's automatic start is Event 6 by design? R47(a) holds the obligation applicable because Ze restarts automatically | In: a second automatic-start path beside the backoff. Out: a recorded deviation, uncounted |
| OQ-5 | Row 8: scope PeerAdj and PeerSet SIDs and per-recipient EPE control in, or out? Neither FRR nor BIRD has them | In: new config, BGP-LS export work, a peer that can receive them. Out: 7-2 and 7-1's gap half recorded out of scope |
| OQ-6 | Row 9: support the type-13 SRv6 BSID backward-compatibility form, or exclude it as the form RFC 9830 §2.4.2 recommends against? | In: encode and decode type 13 with 18 octets. Out: an exclusion quoting the RECOMMENDED sentence |
| OQ-7 | RFC9514-7.2-5 and -6 are canonical `partial` with an unowned gap half ("MUST be set to 0 when originated": no live SRv6 EPE origination path). Should this spec own them too? | Yes: they ride with OQ-5 (same EPE origination work). No: they stay disclosed with no owner |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One spec for ten rows, bucket `immediate/` | Split defects (`immediate/`) from gaps (`plan/`) | Owner asked for one owning spec; the defects decide the bucket, and gap rows act only on his answers |
| RFC9252-5-2 left to `spec-srv6-bestpath-resolvability` | Take it here | It already has a design-status spec with ACs and tags planned; two owners is the defect the pass was cleaning |
| Row 3 withdraws when no interface is known | Keep the bare address | FRR and BIRD both withdraw; a zone-less `fe80::` route is not forwardable |

## Known Limitations
- Rows 6-9 are not implemented by this spec unless the owner scopes them in.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code, using
the quotes in the Task section. Drafts are cited by name and section.

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-bgp-unowned-rfc-rows.md` only, in the same `./le commit create` script

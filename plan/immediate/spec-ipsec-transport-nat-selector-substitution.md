# Spec: ipsec-transport-nat-selector-substitution

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-09-06 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** An operator can set `mode transport` on a site-to-site peer and Ze
negotiates USE_TRANSPORT_MODE, but the peer cannot sit behind an address-translating
NAT. That is the deployment transport mode with NAT traversal exists for. RFC 7296
Section 2.23.1 requires an address substitution on the traffic selectors on BOTH roles
when a NAT is on the path, and Ze performs it on neither.

**The symptom.** A conforming responder answers the substituted selectors, and Ze's
initiator refuses them with TS_UNACCEPTABLE and deletes the SA. A conforming initiator
behind a NAT proposes its pre-NAT address, and Ze's responder narrows that proposal
against a policy naming the observed post-NAT address, finds no intersection, and
answers TS_UNACCEPTABLE. Transport mode behind a real NAT therefore never establishes
in either direction.

**Why the existing evidence did not catch it.** The two NAT-T scenarios landed on
2026-08-30, `natt-transport-inner-checksum` and `natt-tunnel-inner-checksum`, reach the
UDP-encapsulated transport path through strongSwan's `encap = yes`, which fakes the
NAT_DETECTION_SOURCE_IP hash. With no middlebox on the path, the pre-NAT address and
the observed address are equal, so the substitution is the identity and its absence
cannot show. The journal row is in `plan/journal/unwired-feature.md`, dated 2026-08-30.

**Goals.** Each one is a row of the Goal Validation table at closure.

| ID | Goal |
|----|------|
| G-1 | Ze establishes a transport-mode Child SA with a conforming peer across a real address-translating NAT, on the INITIATOR role, and carries traffic over it |
| G-2 | The same, on the RESPONDER role, with the peer behind the NAT |
| G-3 | The narrowing guard still refuses an answer that is genuinely wider than the proposal, with the substitution in place |
| G-4 | Tunnel mode across the same real NAT keeps working, unchanged |
| G-5 | The proof is a red-then-green interop run against strongSwan behind a netfilter NAT, runnable in Docker and in QEMU |

**Owner question (RFC 7296 Section 2.23.1 MAY, `ai/rules/rfc-compliance.md`).** The
section lets a responder that finds no transport-mode policy for the substituted
selectors UNDO the substitution and repeat the lookup for a tunnel-mode entry: "If an
entry is found but it does not allow transport mode, then the server MAY undo the
address substitution and redo the SPD lookup using the original Traffic Selectors."
The three answers are: implement the fallback, skip it and answer TS_UNACCEPTABLE, or
put it behind a config leaf. This spec is not authorized to pick. The question is
OQ-1, and no acceptance criterion below depends on the answer.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - the page `transport_mode.go` and `ts_narrow.go` both declare as their design owner
  → Constraint: the page states transport mode is "an explicit notification, decided per role" and says nothing about NAT. The page edit that adds the substitution lands in the same work as the code, before the next code edit (`ai/rules/documentation.md`)
  → Decision: narrowing has ONE responder entry point on purpose, so the two responder producers cannot drift. The substitution goes inside that entry point rather than beside each caller
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - the responder handshake, design owner of `responder.go`
  → Constraint: NAT detection runs in `detectResponderNAT` during IKE_SA_INIT, so the verdict is on the SA before any selector is read
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - design owner of `child.go`, `dataplane.go` and `xfrm_linux.go`
  → Decision: the Child SA outer addresses come from the operator's `local-address` and `remote-address`, which are the addresses each end's own stack uses, so the substitution changes SELECTORS only and no outer address moves
- [ ] `docs/architecture/testing/interop.md` - the IPsec suite, its scenario discovery and its typed checkers
  → Constraint: a scenario directory carries declarative inputs only; every assertion is a typed Go checker in `internal/le/interoplab/ipsec/checkers.go`
  → Constraint: the directory is NAMED, never numbered, and `interoplab.Discover` matches the name exactly
- [ ] `docs/architecture/testing/qemu-integration.md` - the lab table that pairs each Docker lab with a QEMU action
  → Constraint: the IPsec lab has NO row today. A scenario that adds a netfilter dependency owes the QEMU runner (`ai/rules/platform-linux.md`)
- [ ] `docs/guide/ipsec.md` - the operator page for `mode transport`
  → Constraint: the page tells an operator every prefix must be a single host and says nothing about NAT. It is wrong by omission the day the substitution lands

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - rows RFC7296-2.23.1-1, -2 and -3 declare the three MUSTs of the section, and the `rfc/extraction/rfc7296.json` sites for Section 2.23.1 are all `mapped`
  → Decision: the obligation stays DECLARED in the summary. No `relocated-to-spec` site is created for it (Key Design Decisions, D-4)
  → Constraint: RFC7296-2.23.1-2 reads "The TSi entries MUST have exactly one IP address, and that MUST match the source address of the IKE SA". Behind a NAT the source address of the IKE SA differs between the two ends, and the substitution is what makes the row true at both
- [ ] `rfc/short/rfc3948.md` - UDP encapsulation of ESP, and the checksum problem the substitution's stored originals exist for
  → Constraint: Section 3.1.2's third alternative, ignoring the transport-mode checksum, is what Ze's kernel already takes. `natt-transport-inner-checksum` measures it, so no dataplane change is owed here

**Key insights:** (minimal context to resume after compaction)
- RFC 7296 Section 2.23.1, responder rules, verbatim: "If the client is behind a NAT, substitute the IP address in the TSi entries with the remote address of the IKE SA." and "If the server is behind a NAT, substitute the IP address in the TSr entries with the local address of the IKE SA." and "Do PAD and SPD lookup using the ID and substituted Traffic Selectors." (`rfc/full/rfc7296.txt`, Section 2.23.1, "For the responder, when transport mode is proposed by client").
- RFC 7296 Section 2.23.1, client rules, verbatim: "If the server is behind a NAT, substitute the IP address in the TSr entries with the remote address of the IKE SA." and "If the client is behind a NAT, substitute the IP address in the TSi entries with the local address of the IKE SA." and "Do address substitution before using those Traffic Selectors for anything other than storing original content of them. This includes verification that Traffic Selectors were narrowed correctly by the other end, creation of the SAD entry, and so on."
- RFC 7296 Section 2.23.1 also requires the originals be kept, on both roles: "Store the original Traffic Selectors as the received source and destination address" (client) and "Store the original Traffic Selector IP addresses as received source and destination address, in case undo address substitution is needed, to use as the 'real source and destination address' specified by [UDPENCAPS], and for TCP/UDP checksum fixup" (responder).
- In transport mode there is exactly ONE IP header, and the NAT translates it. ESP authenticates the ESP header and payload, never the IP header, so the translated header reaches the peer intact and its selectors match the substituted pair. That is why no dataplane change is owed.
- RFC 7296 Section 2.15 puts the IKE_SA_INIT payloads under AUTH: the initiator "signs the first message (IKE_SA_INIT request), starting with the first octet of the first SPI in the header and ending with the last octet of the last payload". A tampered NAT_DETECTION notify therefore fails AUTH, so the NAT verdict the substitution reads is authenticated by the time IKE_AUTH completes.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/engine/transport_mode.go` - `transportSelectorPairs` builds a transport-mode PROPOSAL by pinning every configured selector's address to `net.ParseIP(sa.PeerCfg.LocalAddress)` and `net.ParseIP(sa.PeerCfg.RemoteAddress)`, keeping each selector's port and protocol. Its own doc comment claims "The addresses therefore come from the IKE SA rather than from the config", which the code contradicts: it reads the config. Un-NATed the two are equal, so the claim is accidentally true; behind a NAT it is false and the comment is a stale-comment defect fixed in this work
- [ ] `internal/component/ike/engine/ts_narrow.go` - `narrowChildSelectors` is the single responder entry point. It narrows the wire proposal against `policyPairs(sa.PeerCfg, false)`, then applies `keepSingleAddress` when transport mode is in play, then the rekey floor check. `recordInitiatorSelectors` is the initiator adoption path: it decodes, keeps only programmable pairs, then `checkAnswerWithin(pairs, sa.ProposedChildPairs, "the proposal ze sent")` refuses any pair no proposed pair covers, with `errTSWidened`. That refusal is what answers a conforming responder with TS_UNACCEPTABLE today
- [ ] `internal/component/ike/engine/sa.go` - the SA carries `NATDetected` and `BehindNAT`, and the comment on `BehindNAT` reads "true if we are the side behind NAT". There is no field recording that the PEER is behind a NAT: that fact collapses into `NATDetected`, which is also set when THIS node is behind one. `peerEndpoint` holds the source address of the last AUTHENTICATED message, written only by `adoptAuthenticatedEndpoint` after a decrypt and a Message ID window check, and `remoteUDPAddr` falls back to the configured remote when it is nil
- [ ] `internal/component/ike/engine/fsm.go` - the initiator's NAT detection. A NAT_DETECTION_SOURCE_IP mismatch sets `NATDetected` alone (the peer is behind a NAT); a NAT_DETECTION_DESTINATION_IP mismatch sets `NATDetected` and `BehindNAT`
- [ ] `internal/component/ike/engine/responder.go` - `detectResponderNAT` is the mirror, with the same two branches and the same collapse. `matchResponderPeer` accepts an unsolicited IKE_SA_INIT only from a source address equal to the configured `remote-address`, so a `respond` peer behind a NAT is configured with its POST-NAT address and the match already works
- [ ] `internal/component/ike/engine/child.go` - `createFirstChildSA` takes the outer addresses from `sa.PeerCfg.LocalAddress` and `sa.PeerCfg.RemoteAddress` at both call sites, and overwrites `tsLocal`/`tsRemote` with `sa.NegotiatedTSi`/`sa.NegotiatedTSr` in the exchange's orientation. `Selectors: sa.NegotiatedPairs` is what reaches the dataplane. `mode` is `modeTransport` only when `sa.UseTransportMode`
- [ ] `internal/component/ike/engine/rekey.go` - `proposeChildTSPayloads` records `sa.ProposedChildPairs`, and calls `transportSelectorPairs` for a transport-mode peer. `applyChildRekeyResponse` and `respondChildRekey` reach the same two narrowing producers with a floor, so a rekey inherits whatever the two producers do
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - the XFRM encap block sets `Type`, `SrcPort` and `DstPort` and leaves `netlink.XfrmStateEncap.OriginalAddress` unset
- [ ] `internal/le/interoplab/ipsec/ipsec.go` - `scenarioPlan` declares ONE network (`172.28.0.0/24`) and three container names. `prepareScenario` starts strongSwan when `swanctl.conf` exists and FRR when `frr.conf` exists, each at a fixed host octet, and Ze last at octet 2
- [ ] `internal/le/interoplab/docker.go` - `ScenarioPlan.Network` is a single `NetworkSpec` and `runContainer` gives each peer one `--ip` on it. A second network would be an infrastructure change to a package the BGP suite shares
- [ ] `test/interop-ipsec/scenarios/natt-transport-inner-checksum/swanctl.conf` - reaches the transport path with `encap = yes`, whose comment states "The lab has no address-translating middlebox, so this is how the scenario reaches the UDP-encapsulated transport-mode receive path"
- [ ] `test/interop-ipsec/Dockerfile.strongswan` - the peer image already carries `iproute2`, `iptables` and `nmap-nping`
- [ ] `test/interop-ipsec/parity_test.go` - `ipsecScenarios` pins the complete fixture population, so every new directory is named there too
- [ ] `gokrazy/kernel/runtime.config` - carries `CONFIG_IP_NF_NAT=y` and `CONFIG_IP_NF_TARGET_MASQUERADE=y`; `gokrazy/kernel/runtime.require` names neither

**Behavior to preserve:** (unless the user explicitly said to change it)
- Tunnel mode, with and without a NAT: no selector of a tunnel-mode Child SA changes.
- `errTSWidened` still refuses an answer outside the proposal, and `errTSUnusable` still refuses an answer this node cannot decode or program.
- `keepSingleAddress` still requires a /32 on both halves of a transport-mode pair.
- The rekey floor checks in both producers, and their orientation.
- `decideResponderTransportMode`: a responder accepts transport mode only when its own configuration asks for it.
- The two `encap = yes` scenarios and their RFC 3948 assertions.

**Behavior to change:** (only what the user asked for)
- A transport-mode exchange with a NAT detected substitutes the selector addresses on both roles before the selectors are used for anything except storing the originals.
- The SA records that the PEER is behind a NAT, separately from the fact that THIS node is.
- `transportSelectorPairs` takes its addresses from the IKE SA's observed pair rather than from the config, which is what its own comment already claims.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- IKE_SA_INIT NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP notify payloads, read by `detectResponderNAT` (responder) and by the notify loop in `handleSAInitResponse` (initiator). They establish the NAT verdict before any traffic selector is read.
- IKE_AUTH and CREATE_CHILD_SA TSi and TSr payloads, as `wire.PayloadTS`, decrypted and authenticated before either narrowing producer sees them.

### Transformation Path
1. NAT detection writes `NATDetected`, `BehindNAT` and the new `PeerBehindNAT` onto the SA (`fsm.go`, `responder.go`).
2. The initiator builds its proposal: `proposeChildTSPayloads` calls `transportSelectorPairs`, which pins each configured selector to the IKE SA's observed local and remote addresses, and records `sa.ProposedChildPairs`.
3. The responder receives TSi and TSr, and `narrowChildSelectors` substitutes the addresses of the DECODED proposal before it narrows: TSi takes the observed remote address when the peer is behind a NAT, TSr takes the observed local address when this node is. The pre-substitution pairs are stored on the SA.
4. Narrowing, `keepSingleAddress` and the floor check then run on the substituted pairs, unchanged, and `pairsToWire` puts the substituted answer on the wire.
5. The initiator receives that answer, and `recordInitiatorSelectors` substitutes it back before any check: TSi takes the observed local address when this node is behind a NAT, TSr takes the observed remote address when the peer is. The pre-substitution pairs are stored on the SA.
6. `checkAnswerWithin` then tests the substituted answer against the unchanged ceiling `sa.ProposedChildPairs`, and the floor check follows.
7. `createFirstChildSA` reads `sa.NegotiatedPairs`, `NegotiatedTSi` and `NegotiatedTSr`, which now carry the substituted addresses, and installs them.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ engine | `wire.PayloadTS` decoded by `wireToSelectors`, re-encoded by `pairsToWire` | No |
| Engine ↔ dataplane | `ChildSA.Selectors` and `TSLocal`/`TSRemote` to `dp.InstallSA` and the policy install | No |
| Engine ↔ transport | `sa.peerEndpoint` and the socket's bound local address supply the two observed addresses | No |
| Lab ↔ kernel | the NAT container's iptables rules translate the IKE and ESP datagrams on the wire | No |

### Integration Points
- `narrowChildSelectors` (`ts_narrow.go`) - the single responder entry point, reached by `buildAuthResponse`, `startResponderEAP` and `respondChildRekey`. The forward substitution goes inside it, so all three are covered by one edit.
- `recordInitiatorSelectors` (`ts_narrow.go`) - the single initiator adoption path, reached by `adoptAuthResponseNegotiation` and `applyChildRekeyResponse`. The reverse substitution goes inside it.
- `transportSelectorPairs` (`transport_mode.go`) - the proposal builder, reached by `proposeChildTSPayloads`.
- `interoplab.PeerConfig` and `prepareScenario` (`internal/le/interoplab/ipsec/ipsec.go`) - the NAT container joins the existing conditional-peer pattern, keyed on a `nat.conf` in the scenario directory exactly as strongSwan is keyed on `swanctl.conf`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | strongSwan performs the Section 2.23.1 substitution, so it is a conforming counterpart for the red phase | The task statement says a conforming server that performs the substitution draws TS_UNACCEPTABLE from Ze. strongSwan's source was NOT read while this spec was written | The red phase measures Ze against a peer that is also silent, and the scenario proves nothing | Run the scenario before the fix and read charon's log for the selectors it answered; the answered addresses must be the post-NAT pair | unvalidated |
| A-2 | The observed local address of the IKE SA equals the bound listen address, which `ikeListenHost` derives from `interface` or the first peer's `local-address` | `internal/component/ike/engine/register.go` computes one listen host for both sockets | A wildcard bind on a multi-homed host has no per-packet local address, and TSr substitution picks the wrong one | Unit test over the substitution helper with a configured local address; the wildcard case is Known Limitations and belongs to `plan/immediate/spec-rfcgate-1b-rfc7296-pilot-deferred-ike-source-address.md` | unvalidated |
| A-3 | No dataplane change is owed: the kernel takes RFC 3948 Section 3.1.2's third alternative for transport mode, and the substituted selectors match the single translated IP header | `test/interop-ipsec/scenarios/natt-transport-inner-checksum` already measures the checksum behaviour; `xfrm_linux.go` leaves `OriginalAddress` unset | Traffic does not flow over an established transport SA behind the NAT, and `XfrmStateEncap.OriginalAddress` enters scope | The interop scenario asserts the peer's inbound xfrm byte counter advances, which is zero if the selectors or the checksum handling are wrong | unvalidated |
| A-4 | A single netfilter container doing DNAT plus SNAT reproduces the RFC's two-NAT figure well enough to fire all four NAT_DETECTION comparisons. The implemented lab uses SNAT to a named secondary address rather than MASQUERADE, because MASQUERADE picks the interface's primary address and the design needs one distinct public address per peer | RFC 7296 Section 2.23.1 describes NAT A and NAT B as two boxes for exposition; the hashes compare addresses, not box counts | Only one substitution arm is exercised and the other ships unproven | The checker asserts `nat-detected` on Ze's `show vpn ipsec sa` and asserts the negotiated selectors carry the post-NAT addresses on both roles | unvalidated |
| A-5 | Docker's user-defined bridge forwards through a container that owns a secondary address and masquerades, with no anti-spoof filter in the way | MASQUERADE rewrites the source to an address the NAT container owns, so no spoofed source is ever emitted | The lab cannot introduce a NAT without a second Docker network, which is an `interoplab` change the BGP suite shares | Bring the three containers up and ping across the NAT before any IKE runs | unvalidated |
| A-6 | The Alpine QEMU VM can run strongSwan from `apk`, and Ze's runtime kernel carries iptables NAT | `test/interop-ipsec/Dockerfile.strongswan` installs `strongswan` from Alpine; `gokrazy/kernel/runtime.config` carries `CONFIG_IP_NF_NAT=y` and `CONFIG_IP_NF_TARGET_MASQUERADE=y` | The QEMU runner needs kernel config work before it can run at all | Boot the runtime kernel and install the masquerade rule in the middle namespace | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Loosening the answer check opens a selector-confusion hole: a peer names an address and Ze installs it | A unit test that answers an address outside the proposal passes | The peer's asserted address is DISCARDED, never trusted. The substitution replaces it with an address this node observed: its own bound local address, or `sa.peerEndpoint`, which only `adoptAuthenticatedEndpoint` writes and only after a decrypt and a window check. The ceiling itself is unchanged, and AC-6 is the negative test |
| R-2 | The substitution leaks into tunnel mode and changes selectors an operator configured | A tunnel-mode scenario's negotiated selectors move | The substitution is gated on transport mode AND a NAT verdict, and `real-nat-tunnel-control` is the scenario that fails if it leaks |
| R-3 | The substitution runs on a rekey with a floor, and the floor is in pre-substitution addresses while the new pairs are in post-substitution ones, so `coversFloor` refuses every rekey | The tunnel drops one lifetime after establishment | The floor is `sa.NegotiatedPairs` of the SA being replaced, which is already substituted, so both sides of the comparison are in the same space. `child-rekey` over the NAT topology is the test |
| R-4 | `PeerBehindNAT` is added but one of the two detection sites is missed, so one role substitutes and the other does not | One direction establishes and the other answers TS_UNACCEPTABLE | Both scenarios run, one per role, and a unit test asserts the field at each of the four detection branches |
| R-5 | The NAT container's conntrack drops the ESP-in-UDP flow under the default UDP timeout during a long scenario | The SA establishes and traffic stops later | NAT-T keepalives already run on an established SA (`established.go`); the checker asserts traffic after the keepalive interval |
| R-6 | The QEMU runner is built, registered, and called by nothing, so it stays green and runs nowhere | `./le qemu` lists the action and no workflow names it | `platform-linux.md` requires a real caller in the same change: name the workflow job in the same commit |
| R-7 | The red phase is faked: the scenario is added after the fix and never observed red | No RED output is pasted at closure | AC-9 makes the recorded RED a criterion, with the container image rebuilt so the revert takes effect |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Transport-mode Child SAs negotiate the wrong selectors and the dataplane drops every packet, or a peer chooses the traffic Ze protects. Tunnel mode is untouched by construction; the gate is transport mode plus a NAT verdict |
| How is it reverted? | Single commit revert. Nothing persists: no config leaf, no schema change, no on-disk state |
| Who else touches this path? | `plan/spec-ipsec-11-mobike.md` (address updates on an established SA), `plan/spec-ipsec-esp-dual-form-receive.md` (the ESP form logic beside it), and `plan/immediate/spec-rfcgate-1b-rfc7296-pilot-deferred-ike-source-address.md` (the local-address question A-2 names) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `mode transport` peer, IKE_AUTH answered by a peer behind a NAT | → | `recordInitiatorSelectors` reverse substitution | `TestInitiatorAdoptsSubstitutedTransportSelectorsBehindNAT` |
| `connection-type respond` peer, IKE_AUTH request from behind a NAT | → | `narrowChildSelectors` forward substitution | `TestResponderSubstitutesTransportSelectorsBeforeNarrowing` |
| IKE_SA_INIT NAT_DETECTION_SOURCE_IP mismatch on either role | → | `PeerBehindNAT` written at all four detection branches | `TestNATDetectionRecordsWhichSideIsBehindTheNAT` |
| `proposeChildTSPayloads` for a transport peer | → | `transportSelectorPairs` reading the observed IKE SA pair | `TestTransportProposalUsesObservedIKEAddresses` |
| `./le integration interop-ipsec scenario real-nat-transport-ze-initiator` | → | the whole path, against strongSwan across a netfilter NAT | `real-nat-transport-ze-initiator` checker in `internal/le/interoplab/ipsec/checkers.go` |
| `./le qemu ipsec-nat-transport-test` | → | the same path on Ze's runtime kernel, three network namespaces | `ipsec-nat-transport-test` native action |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Transport mode, NAT detected, this node is the RESPONDER and the peer is behind the NAT | The TSi addresses used for narrowing are the observed remote address of the IKE SA, and the answer put on the wire carries them |
| AC-2 | Transport mode, NAT detected, this node is the RESPONDER and behind the NAT itself | The TSr addresses used for narrowing are the observed local address of the IKE SA, and the answer carries them |
| AC-3 | Transport mode, NAT detected, this node is the INITIATOR and behind the NAT; the responder answers the post-NAT pair | The answer is accepted, the installed TSi is the observed local address, and no TS_UNACCEPTABLE is sent |
| AC-4 | Transport mode, NAT detected, this node is the INITIATOR and the peer is behind the NAT | The installed TSr is the observed remote address of the IKE SA |
| AC-5 | Any of AC-1 through AC-4 | The pre-substitution TSi and TSr addresses are stored on the SA and are readable after the Child SA installs |
| AC-6 | Transport mode, NAT detected, initiator; the responder answers an address that is neither the observed local nor the observed remote address, and lies outside the proposal | The answer is refused with `errTSWidened` and the peer is sent TS_UNACCEPTABLE, exactly as before this change |
| AC-7 | Tunnel mode with a NAT detected, and transport mode with NO NAT detected | Every negotiated selector is identical to what the same exchange produced before this change |
| AC-8 | `mode transport` peer whose Child SA rekeys across the NAT | The rekey is accepted, and the floor comparison runs with both sides in post-substitution addresses |
| AC-9 | The fix is reverted, the Ze container image is rebuilt, and `real-nat-transport-ze-initiator` and `real-nat-transport-ze-responder` are run | Both scenarios FAIL, and the failure output is recorded in the spec before the fix is restored |
| AC-10 | `./le integration interop-ipsec` with the fix in place | `real-nat-transport-ze-initiator`, `real-nat-transport-ze-responder` and `real-nat-tunnel-control` all pass, and each asserts the peer's inbound xfrm byte counter advanced |
| AC-11 | `./le qemu ipsec-nat-transport-test` on Ze's runtime kernel | Ze and strongSwan establish transport mode across a masquerading namespace, and traffic crosses it |
| AC-12 | `./le qemu` with no arguments, and the lab table in `docs/architecture/testing/qemu-integration.md` | The new action is listed in both, and a workflow job names it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `mode transport` on a peer whose Ze node sits behind a NAT, and reaches the far host | config → `proposeChildTSPayloads` → IKE_AUTH → `recordInitiatorSelectors` → `createFirstChildSA` → XFRM | `real-nat-transport-ze-initiator` |
| 2 | Configures `mode transport` with `connection-type respond` and accepts a peer that dials in from behind a NAT | inbound IKE_SA_INIT → `matchResponderPeer` → `detectResponderNAT` → `narrowChildSelectors` → `createFirstChildSA` → XFRM | `real-nat-transport-ze-responder` |
| 3 | Runs `show vpn ipsec sa` on either node and reads the selectors the tunnel really carries | `internal/component/ike/cmd/show_ipsec.go` → the SA's negotiated pairs | `real-nat-transport-ze-initiator` checker, which reads the command |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNATDetectionRecordsWhichSideIsBehindTheNAT` | `internal/component/ike/engine/nat_detect_test.go` | All four detection branches write `PeerBehindNAT` and `BehindNAT` independently | |
| `TestTransportProposalUsesObservedIKEAddresses` | `internal/component/ike/engine/rfc7296_transport_test.go` | `transportSelectorPairs` pins to the observed pair, and ports and protocols survive | |
| `TestResponderSubstitutesTransportSelectorsBeforeNarrowing` | `internal/component/ike/engine/ts_nat_substitute_test.go` | AC-1 and AC-2, including that `policyPairs` is matched against the substituted pair | |
| `TestInitiatorAdoptsSubstitutedTransportSelectorsBehindNAT` | `internal/component/ike/engine/ts_nat_substitute_test.go` | AC-3 and AC-4 | |
| `TestSubstitutionStoresTheOriginalSelectorAddresses` | `internal/component/ike/engine/ts_nat_substitute_test.go` | AC-5 on both roles | |
| `TestSubstitutionStillRefusesAWidenedAnswer` | `internal/component/ike/engine/ts_initiator_subset_test.go` | AC-6: the ceiling is unchanged and `errTSWidened` still fires | |
| `TestTunnelModeSelectorsUnchangedWithNATDetected` | `internal/component/ike/engine/ts_nat_substitute_test.go` | AC-7, both arms of the gate | |
| `TestChildRekeyFloorComparedInSubstitutedSpace` | `internal/component/ike/engine/child_rekey_initiator_answer_test.go` | AC-8 | |
| `TestIPsecScenarioParity` (existing) | `test/interop-ipsec/parity_test.go` | The three new directories are in `ipsecScenarios` and have checkers | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Substituted selector prefix length | 32 only (IPv4 host) | 32 | 31, refused by `keepSingleAddress` | N/A |
| Selectors per transport-mode pair set | 1..n, all sharing one address | n | 0, refused as `errTSUnacceptable` | N/A |
| NAT container host octet on `172.28.0.0/24` | 2..254 | 5 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-transport-nat-selectors` | `test/ipsec/ipsec-transport-nat-selectors.ci` | An operator brings up a transport-mode peer whose SA reports the post-NAT selectors in `show vpn ipsec sa`, with `option=needs-linux` and the capability the XFRM install needs | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `real-nat-transport-ze-initiator` | `test/interop-ipsec/scenarios/` | strongSwan | Ze initiates transport mode from behind a real netfilter NAT, adopts the substituted answer, and carries traffic | |
| `real-nat-transport-ze-responder` | `test/interop-ipsec/scenarios/` | strongSwan | Ze answers a transport-mode request from a peer behind a real NAT, substitutes before narrowing, and carries traffic | |
| `real-nat-tunnel-control` | `test/interop-ipsec/scenarios/` | strongSwan | Tunnel mode over the identical NAT topology is unchanged, which is the control that makes the two transport verdicts readable | |

## Files to Modify
- `internal/component/ike/engine/sa.go` - add `PeerBehindNAT`, and the two stored original-selector fields
- `internal/component/ike/engine/fsm.go` - set `PeerBehindNAT` at the initiator's NAT_DETECTION_SOURCE_IP branch
- `internal/component/ike/engine/responder.go` - set `PeerBehindNAT` in `detectResponderNAT`
- `internal/component/ike/engine/rekey.go` - carry `PeerBehindNAT` across the two rekey copies that already carry `NATDetected` and `BehindNAT`
- `internal/component/ike/engine/transport_mode.go` - `transportSelectorPairs` reads the observed IKE SA pair, and its stale doc comment is corrected
- `internal/component/ike/engine/ts_narrow.go` - call the substitution from `narrowChildSelectors` and from `recordInitiatorSelectors`
- `internal/component/ike/cmd/show_ipsec.go` - report which side is behind the NAT, beside the existing `nat-detected`
- `internal/le/interoplab/ipsec/ipsec.go` - the conditional NAT container, its image, and the `nat.conf` parse
- `internal/le/interoplab/ipsec/checkers.go` - three typed checkers
- `internal/le/interoplab/ipsec/helpers.go` - a helper that reads the negotiated selectors from both daemons
- `internal/le/qemu/actions.go` - register `ipsec-nat-transport-test`
- `internal/le/qemu/alltests.go` - the integration package, if the runner adds one
- `test/interop-ipsec/parity_test.go` - the three new scenario names
- `gokrazy/kernel/runtime.require` - require `CONFIG_IP_NF_NAT` and `CONFIG_IP_NF_TARGET_MASQUERADE` so a demotion to `=m` fails the build
- `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - the substitution, its gate and its safety argument
- `docs/architecture/ike/ipsec-14-responder.md` - the responder's forward substitution before the policy match
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - that the installed selectors are the substituted ones and the outer addresses are not
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the NAT verdict fields the SA now carries
- `docs/architecture/ike/ipsec-10-cli-diag.md` - the design owner of `show_ipsec.go`, whose SA payload gains the new field
- `docs/architecture/testing/interop.md` - the NAT container in the IPsec topology
- `docs/architecture/testing/qemu-integration.md` - the IPsec row in the lab table
- `docs/guide/ipsec.md` - what an operator writes for a transport peer behind a NAT
- `docs/features/rfc-status.md` - the RFC 7296 row, whose Section 2.23.1 sentence currently says Ze "pins TSi and TSr to the IKE SA's own address pair" with no mention of NAT
- `rfc/short/rfc7296.md` - the Section 2.23.1 rows, now proven behind a real NAT, with the scenario named

## Files to Create
- `internal/component/ike/engine/ts_nat_substitute.go` - the two substitution producers and the observed-address accessors
- `internal/component/ike/engine/ts_nat_substitute_test.go` - the unit tests above
- `internal/component/ike/engine/nat_detect_test.go` - the detection-branch test
- `internal/le/qemu/ipsec_nat_linux.go` - the three-namespace QEMU runner
- `test/interop-ipsec/Dockerfile.nat` - alpine with `iproute2` and `iptables`, entrypoint installs the rules and stays up
- `test/interop-ipsec/scenarios/real-nat-transport-ze-initiator/` - `ze.conf`, `swanctl.conf`, `strongswan.conf`, `nat.conf`
- `test/interop-ipsec/scenarios/real-nat-transport-ze-responder/` - `ze.conf`, `swanctl.conf`, `strongswan.conf`, `nat.conf`
- `test/interop-ipsec/scenarios/real-nat-tunnel-control/` - `ze.conf`, `swanctl.conf`, `strongswan.conf`, `nat.conf`
- `test/ipsec/ipsec-transport-nat-selectors.ci` - the functional test above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No new config. `mode transport` and `transport-required` already exist; the substitution is protocol behaviour, not an operator choice |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | `show vpn ipsec sa` gains a field, not a command |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-transport-nat-selectors.ci` |
| Pipe completeness | N-A | The changed output is an existing command's payload, already routed through the pipe machinery |
| Env var registration | N-A | No environment leaf |
| Doctor check for runtime dependencies | N-A | The engine gains no file, socket, port, module or binary. The NAT dependency is the LAB's, and `ai/rules/platform-linux.md` covers it through `runtime.require` rather than a doctor check |
| Prometheus counters/metrics | No | No new counter; the interop assertion reads the kernel's own xfrm counters |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- transport mode works behind a NAT |
| 2 | Config syntax changed? | No | No leaf added or changed |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- the new field of `show vpn ipsec sa` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- the same field in the JSON payload |
| 5 | Plugin added/changed? | N-A | IKE is a component, not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`, the Transport mode section |
| 7 | Wire format changed? | No | The TS payload encoding is unchanged; only the addresses it carries change |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7296.md` Section 2.23.1 rows, and the `docs/features/rfc-status.md` RFC 7296 row, each with a source anchor |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/interop.md`, `docs/architecture/testing/qemu-integration.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- transport mode behind NAT is a feature other daemons carry |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md`, `docs/architecture/ike/ipsec-14-responder.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-7-ikev2-engine.md`, `docs/architecture/ike/ipsec-10-cli-diag.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` -- the `./le qemu` action inventory gains `ipsec-nat-transport-test` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/immediate/spec-ipsec-transport-nat-selector-substitution.md`. The `// Design:` owners of the changed files are `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` (transport_mode.go, ts_narrow.go), `docs/architecture/ike/ipsec-7-ikev2-engine.md` (sa.go, fsm.go), `docs/architecture/ike/ipsec-14-responder.md` (responder.go), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (child.go, xfrm_linux.go), `docs/architecture/ike/ipsec-10-cli-diag.md` (show_ipsec.go), `docs/architecture/testing/interop.md` (the three interoplab files) and `docs/architecture/core-design.md` (qemu actions.go). Each is named above or, for core-design, is unaffected because the qemu table gains a row and its dispatch is unchanged |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` shows a `mode transport` example whose selectors are host prefixes; verify it still reads correctly beside a NAT paragraph |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the NAT verdict complete and reachable
   - Tests: `TestNATDetectionRecordsWhichSideIsBehindTheNAT`
   - Files: `sa.go`, `fsm.go`, `responder.go`, `rekey.go`, `show_ipsec.go`
   - Verify: the field exists, all four detection branches write it, both rekey copies carry it, and the CLI reports it. The substitution tests still fail because no substitution exists
2. **Phase: The substitution producers** -- one file, two directions, no call sites yet
   - Tests: `TestResponderSubstitutesTransportSelectorsBeforeNarrowing`, `TestInitiatorAdoptsSubstitutedTransportSelectorsBehindNAT`, `TestSubstitutionStoresTheOriginalSelectorAddresses`
   - Files: `ts_nat_substitute.go`
   - Verify: each producer is exercised directly, and the RFC quotes sit above the code that enforces them
3. **Phase: Wire the producers into the two entry points**
   - Tests: `TestSubstitutionStillRefusesAWidenedAnswer`, `TestTunnelModeSelectorsUnchangedWithNATDetected`, `TestChildRekeyFloorComparedInSubstitutedSpace`, `TestTransportProposalUsesObservedIKEAddresses`
   - Files: `ts_narrow.go`, `transport_mode.go`
   - Verify: AC-1 to AC-8 pass, and the gate keeps tunnel mode unchanged
4. **Phase: The NAT lab** -- container, image, scenario plumbing
   - Tests: `TestIPsecScenarioParity`, plus the three scenarios brought up and pinged before any IKE runs
   - Files: `Dockerfile.nat`, `ipsec.go`, the three scenario directories, `parity_test.go`
   - Verify: the topology carries traffic through the NAT with no IPsec involved, which is A-5
5. **Phase: The red phase (BLOCKING, AC-9)** -- revert phases 2 and 3, rebuild the Ze image, run the two transport scenarios, record the failure output, restore
   - Tests: the two transport scenarios
   - Files: the spec, which records the RED output
   - Verify: both go RED for the stated reason, and `real-nat-tunnel-control` stays GREEN, which proves the topology is not what fails
6. **Phase: The checkers and the green run**
   - Tests: `real-nat-transport-ze-initiator`, `real-nat-transport-ze-responder`, `real-nat-tunnel-control`
   - Files: `checkers.go`, `helpers.go`
   - Verify: AC-10, each scenario asserting the peer's inbound xfrm byte counter advanced
7. **Phase: The QEMU runner**
   - Tests: `./le qemu ipsec-nat-transport-test`
   - Files: `ipsec_nat_linux.go`, `actions.go`, `alltests.go`, `runtime.require`, the workflow job that calls it
   - Verify: AC-11 and AC-12, on Ze's runtime kernel, with the action listed and called
8. **Phase: Documentation** -- every page named in the two checklists, in this work and not at closure
   - Files: the documentation paths in Files to Modify
   - Verify: `./le verify lint run` and the citation-anchor audit are clean

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and AC-9's RED output is pasted rather than described |
| Feature completeness | Both roles substitute; a fix on one role only is the failure R-4 names |
| Correctness | The substituted address is one this node OBSERVED, never one the peer asserted. Read `adoptAuthenticatedEndpoint` and confirm no unauthenticated write reaches `peerEndpoint` |
| Correctness | The gate is transport mode AND a NAT verdict. Confirm no path reaches the substitution with `UseTransportMode` false and `PeerRequestedTransport` false |
| Naming | `PeerBehindNAT` reads as "the peer is behind a NAT" and `BehindNAT` keeps its documented meaning. Neither is read for the other (`plan/journal/field-carries-two-meanings.md`) |
| Data flow | The originals are stored BEFORE the substitution, on both roles, which is what the RFC's own ordering requires |
| Rule: `ai/rules/interop-and-goal-validation.md` | The RED was forced with the container image rebuilt, not with a source edit the image never saw |
| Rule: `ai/rules/platform-linux.md` | The QEMU action has a real caller, and the kernel symbols it needs are in `runtime.require` |
| Rule: `ai/rules/stale-comments.md` | The `transportSelectorPairs` comment that claims the addresses come from the IKE SA now matches the code |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Substitution on both roles | `go test ./internal/component/ike/engine/ -run Substitut` |
| Three interop scenarios | `./le integration interop-ipsec scenario real-nat-transport-ze-initiator`, and the same for the other two |
| The recorded RED | The closure half carries the pasted failure output of the two transport scenarios |
| The QEMU action | `./le qemu` lists `ipsec-nat-transport-test`, and a workflow job names it |
| The scenario population | `go test ./test/interop-ipsec/ -run TestIPsecScenarioParity` |
| Every documentation page | `./le spec citation anchors spec plan/immediate/spec-ipsec-transport-nat-selector-substitution.md` reports no unnamed owner |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The peer's asserted selector address is discarded, not clamped. Confirm no path admits it |
| Authorization that could fail open | `checkAnswerWithin` must still refuse. A ceiling that becomes empty because the substitution emptied `ProposedChildPairs` would admit everything |
| Untrusted input | `peerEndpoint` is the only peer-derived address used, and only `adoptAuthenticatedEndpoint` writes it, after a decrypt and a Message ID window check |
| Resource exhaustion | The substitution allocates one pair slice per exchange, bounded by the selector count the wire already bounds |
| Error leakage | The refusal messages name addresses, which are already on the wire |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| The scenario stays RED after the fix | Read charon's log for the selectors it answered. If strongSwan did not substitute, A-1 is broken and the peer choice goes back to the user |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- In transport mode there is exactly one IP header and the NAT translates it, while ESP authenticates only the ESP header and payload. That is why the substituted selectors match what arrives, and why no dataplane change is owed. Reasoning about an inner and an outer header here produces a design that is wrong in both directions.
- The two existing NAT-T scenarios cannot fail on this defect by construction: with no middlebox the substitution is the identity. A scenario whose fixture makes the mechanism a no-op is the "test whose data reaches the peer by a different path" trap `docs/architecture/testing/interop.md` names.
- `NATDetected` answers "is there a NAT", not "which side". Two of the four RFC substitution rules need "which side", so the missing field is not an optimisation; it is the fact the rule is written in terms of.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: substitute INSIDE `narrowChildSelectors` and `recordInitiatorSelectors` | Substitute at each of the five call sites; substitute at the wire decode | The two functions are already the single entry points for their roles, and `narrowChildSelectors`'s own comment says it exists so the two responder producers cannot drift. Five call sites reintroduces exactly the drift it was written to prevent, and the EAP responder path is the one that gets forgotten |
| D-2: one NAT container doing DNAT plus MASQUERADE on the existing flat `172.28.0.0/24` | Two Docker networks with a router; netns plus masquerade inside a peer container | `ScenarioPlan.Network` is a single `NetworkSpec` and `runContainer` gives one `--ip`, so a second network is an `interoplab` change the BGP suite shares. One box with a secondary address answers ARP for the alias and needs no route on either peer, and it fires all four NAT_DETECTION comparisons because both the source and the destination are translated |
| D-3: iptables, not nftables, in both the container and the QEMU namespace | nftables in both; nftables in Docker and iptables in QEMU | The strongSwan image already carries `iptables`, and `gokrazy/kernel/runtime.config` already carries `CONFIG_IP_NF_NAT` and `CONFIG_IP_NF_TARGET_MASQUERADE` while carrying no NFT NAT symbol. One mechanism in both paths means the two proofs cannot diverge |
| D-4: the Section 2.23.1 obligation stays DECLARED in `rfc/short/rfc7296.md`; no `relocated-to-spec` site is created | Relocate the three sites to this spec with `reserved-id` | All three Section 2.23.1 sites in `rfc/extraction/rfc7296.json` carry `disposition: mapped` today, and the summary declares the rows. `rfc/extraction/README.md` makes a relocation an exclusion for the ratchet, so turning a mapping into one costs a `resign-reason` and a bumped `signed-off` on the largest signed artifact in the tree, and RAISES the published exclusion ratio for rfc7296. The twelve existing relocations were moved OUT of the summary by owner ruling D-1 of 2026-07-31; no ruling covers this section, and the obligation never left. Relocating would pay a ratchet cost to move a requirement that is already in the right place |
| D-5: no `XfrmStateEncap.OriginalAddress` in this slice | Set it from the stored originals | The field is unset today, and the checksum behaviour it would serve is already measured by `natt-transport-inner-checksum`, which shows Ze's kernel taking RFC 3948 Section 3.1.2's third alternative. Adding a netlink field with no observable difference is machinery `ai/rules/simplicity.md` cuts. A-3 is the assumption that keeps this honest, and the scenario's byte counter is what would break it |
| D-6: three scenarios, not one | One scenario with both roles; two scenarios with no tunnel control | The two roles are two code paths and a single scenario cannot fail on one of them. The tunnel control is what makes the transport verdicts readable: without it a red transport scenario is equally explained by a broken NAT topology |

## Known Limitations

**Not implemented in the commit that set Status to `verification`.** The
implementation session landed AC-1 through AC-8 and the three interop scenarios, and
left the rest open. It reduced no acceptance criterion: each item below is still
owed and each one names what remains.

- AC-9 and AC-10 are UNRUN, not unwritten. `real-nat-transport-ze-initiator`,
  `real-nat-transport-ze-responder` and `real-nat-tunnel-control` are checked in with
  their typed checkers, their `nat.conf` fixtures and the NAT container that serves
  them, and no run of the container lab has been made against them: the owner deferred
  heavy testing for that session. The commands are
  `./le integration interop-ipsec scenario real-nat-transport-ze-initiator` and the
  same for the other two, and AC-9's recorded RED still has to be forced with the Ze
  image rebuilt.
- AC-11 and AC-12 are NOT STARTED. `internal/le/qemu/ipsec_nat_linux.go`, its
  registration in `internal/le/qemu/actions.go`, the `runtime.require` entries and the
  workflow job that calls it do not exist.
- The `.ci` functional test `test/ipsec/ipsec-transport-nat-selectors.ci` does not
  exist.
- The IKE SA rekey carries `PeerBehindNAT` across both producers in `rekey.go`, and no
  test drives either producer: the two build a replacement SA from keys, nonces and a
  Diffie-Hellman exchange, and no fixture in the package reaches them today. The
  CHILD SA rekey is covered, by `TestChildRekeyFloorComparedInSubstitutedSpace`.

- A wildcard IKE bind on a multi-homed host has no per-packet local address, so the TSr substitution uses the bound listen address. `plan/immediate/spec-rfcgate-1b-rfc7296-pilot-deferred-ike-source-address.md` owns that question and is `blocked` on owner question OR-WP8-1. This spec does not widen its scope, and A-2 records the boundary.
- IPv6 transport mode behind a NAT is out of scope: the IKE transport is `udp4` today, so there is no IPv6 path to substitute on.
- The VPP dataplane backend refuses transport mode, so this work is XFRM-only in effect.
- OQ-1, the Section 2.23.1 tunnel-mode fallback MAY, is unanswered. No acceptance criterion depends on it.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

The four substitution rules of RFC 7296 Section 2.23.1, two per role, are quoted
verbatim above the code that performs each one, together with the storage rule for the
originals and the "Do address substitution before using those Traffic Selectors for
anything other than storing original content of them" ordering sentence. The
`transportSelectorPairs` comment keeps its RFC7296-2.23.1-2 and -3 quotes and stops
claiming a provenance the code did not have.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket
- [ ] OQ-1 answered by the owner, and the answer recorded

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
- [ ] The interop RED phase recorded, with the container image rebuilt so the revert took effect

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** remove the spec only, since commit A preserves it in history

## Progress, 2026-09-06

Committed in `2e5da6d39`. The QEMU runner that two of the scenarios need does
not exist.

**The three interop scenarios RAN for the first time on 2026-09-06, and all
three FAILED.** They had been unrunnable because the lab's container image
compiled `ze` inside itself and the build was killed by the kernel for memory;
`5837fd3247` made the image copy a prebuilt binary, taking the build from
impossible here to 54.7 seconds. Command:
`INTEROP_SCENARIO=real-nat-tunnel-control ./le integration interop-ipsec`, which
runs the whole suite — 21 passed, 8 failed.

The symptom is specific and it is the assertion these scenarios exist to make:

```
show vpn ipsec sa does not report nat-detected true
show vpn ipsec sa does not report peer-behind-nat true
```

`real-nat-transport-ze-initiator`, `real-nat-transport-ze-responder` and
`real-nat-tunnel-control` all fail this way. **The columns are present** in the
answer — `behind-nat`, `nat-detected` and `peer-behind-nat` all appear in the
header — so the fields this spec added exist and reach the operator surface;
what does not happen is the values becoming true.

Not diagnosed, and deliberately so: whether the NAT container fails to rewrite
the addresses, or `NATDetected` genuinely never sets under that topology, is two
different defects with two different fixes, and picking one from the failure text
is the mistake `plan/journal/` records repeatedly. The next step is one
measurement — read the payload the container actually rewrites, then the four
NAT_DETECTION comparisons — and it decides which.

Five other scenarios fail in the same run (`child-rekey-narrowing`,
`delete-while-window-held`, `initiator-rekey-answer-narrows`,
`ipsec-bgp-redistribute-frr`, `peer-reload-narrowing`). They are NOT attributed
to this spec and were not examined; whether they predate it is unmeasured.

## Progress, 2026-09-06, second session: the eight failures are diagnosed and six are green

**The daemon was right and the CHECKER was wrong.** Every value the three NAT
scenarios assert was already true. The failing run's own error message carries
`behind-nat true`, `nat-detected true` and `peer-behind-nat true` on the SA row,
so the NAT box translates, all four NAT_DETECTION comparisons mismatch, and the
substitution runs. What could not happen was the ASSERTION matching.

The producer is `assertZeSAField` and `assertZeSelectors`
(`internal/le/interoplab/ipsec/helpers.go`), which read the TEXT rendering of
`show vpn ipsec sa` with `(?m)^<field>\s+<value>\s*$`. That answer is a table.
`applyTableStyled` (`internal/component/command/pipe.go`) orders its columns by
field name, and a nested `child-sa` key starts a line only while `child-sa` holds
the first column. `nat-detected` is a header cell and `true` is a row cell, so the
NAT verdict could never match at all.

**The fix is at the observation.** `zeIKESAs` asks
`show vpn ipsec sa | json` and decodes the SA records, so each assertion reads the
field the command produced rather than a cell the renderer placed. Both
assertions now hold over ONE record, which is stronger than the regex was: a
`ts-local` from one Child SA and a `ts-remote` from another described a tunnel
that does not exist. The rule is on `docs/architecture/testing/interop.md` and the
row is in `plan/journal/green-that-could-not-have-been-red.md`.

**Verdicts after the fix, one scenario per run.**

| Scenario | Before | After | Attribution |
|----------|--------|-------|-------------|
| `real-nat-transport-ze-initiator` | FAIL | PASS | the checker, never the daemon |
| `real-nat-transport-ze-responder` | FAIL | PASS | the checker, never the daemon |
| `real-nat-tunnel-control` | FAIL | PASS | the checker, never the daemon |
| `child-rekey-narrowing` | FAIL | PASS | this spec, through the checker: `behind-nat` sorts ahead of `child-sa` and moved every nested key off the line start |
| `peer-reload-narrowing` | FAIL | PASS | the same |
| `initiator-rekey-answer-narrows` | FAIL | PASS | the same |
| `delete-while-window-held` | FAIL | FAIL | PRE-EXISTING, measured |
| `ipsec-bgp-redistribute-frr` | FAIL | FAIL | PRE-EXISTING, measured |

The two pre-existing failures are `wait for strongswan log "received DELETE for
ESP CHILD_SA" timed out` and `wait for FRR route 10.200.0.0/24 timed out`. They
fail identically against a tree carrying this spec's seven engine and CLI files
reverted to `2e5da6d394^`, so this spec did not cause them. Neither scenario
configures transport mode or a `nat.conf`, and `substituteResponderSelectors`
and `substituteInitiatorSelectors` return their input unless
`transportSelectorsInPlay` and `sa.NATDetected` are both true
(`internal/component/ike/engine/ts_nat_substitute.go`), so no code this spec added
runs in either. `2e5da6d394..HEAD` carries another spec over
`internal/component/ike/` (RFC 4301 SPD discard, `xfrm_linux.go`,
`engine/register.go`), which is where a reader should look next.

**AC-9, the recorded RED, is now taken.** The same three scenarios were run
against a tree holding this spec's engine and CLI files reverted, the Ze image
rebuilt from it, and the fixed checker in place:

```
real-nat-transport-ze-initiator  FAIL: wait for strongSwan SA ze timed out before the peer became ready
real-nat-transport-ze-responder  FAIL: wait for strongSwan SA ze timed out before the peer became ready
real-nat-tunnel-control          FAIL: show vpn ipsec sa reports no IKE SA with nat-detected, behind-nat, peer-behind-nat all true
```

Without the substitution the transport-mode Child SA never establishes across the
real NAT, on either role, which is the failure this spec exists to remove. The
control fails on the verdict fields alone, which is what it is for: its selectors
are inner addresses no translation moves.

**Where the runs were made, and why it matters.** The main working tree cannot
build `le` today: `internal/component/bgp/reactor/peer.go` names an undefined
`staticWireSet` from another session's live edit. Each run therefore used a tree
extracted from HEAD with only this change applied. HEAD does not compile either,
because `2e5da6d394` carried an unrelated `cfg.CRLPEM = ca.CRLPEM()` hunk into
`fsm.go` whose producers are still another session's uncommitted work; that hunk
was removed in the scratch copy alone and the row is in
`plan/journal/concurrent-session-corruption.md`.

**Still open.** AC-11 and AC-12 (the QEMU runner), the `.ci` test, and OQ-1 are
untouched by this session. AC-10's byte-counter half is met: each of the three
checkers ends in `verifyESPDirectionsToward` over `natESPDirections`, and
`assertESPAdvanced` (`internal/le/interoplab/ipsec/helpers.go`) refuses unless the
XFRM byte counter of a surviving SPI GREW across the ping, in all four simplex
directions, `swanDecryptsNAT` and `zeDecryptsNAT` among them.

---

## Implementation Summary

### What Was Implemented
- The RFC 7296 Section 2.23.1 substitution on both roles, in `internal/component/ike/engine/ts_nat_substitute.go`, called from the single entry point of each role: `substituteResponderSelectors` inside `narrowChildSelectors` above the policy match that is Ze's SPD lookup, and `substituteInitiatorSelectors` inside `recordInitiatorSelectors` above every check.
- `SA.PeerBehindNAT` beside `BehindNAT`, written at all four NAT_DETECTION branches (`detectResponderNAT`, `handleSAInitResponse`) and carried across both IKE SA rekey producers (`applyIKERekeyResponse`, `respondIKERekey`).
- `SA.OriginalTSiAddr` and `SA.OriginalTSrAddr`, stored before the substitution on both roles.
- `transportSelectorPairs` reads the IKE SA's observed pair through `observedLocalAddress` and `observedRemoteAddress` rather than the config, which is what its own doc comment already claimed.
- `show vpn ipsec sa` reports `behind-nat`, `peer-behind-nat` and, added at closure, `original-tsi` and `original-tsr` (`saToMap`, `selectorAddressText`).
- The interop lab's address-translating NAT box, keyed on a scenario `nat.conf` (`internal/le/interoplab/ipsec/nat.go`), and the three scenarios `real-nat-transport-ze-initiator`, `real-nat-transport-ze-responder` and `real-nat-tunnel-control` with their typed checkers.

### Bugs Found/Fixed
- **The checkers asserted on the rendered table, so they could not match.** `assertZeSAField` and `assertZeSelectors` read the TEXT rendering of `show vpn ipsec sa` with a line-anchored `<field> <value>` regex. `applyTableStyled` orders columns by field name, so `behind-nat` took first place from `child-sa` and moved every nested key off the line start, and `nat-detected` was a header cell whose `true` was a row cell. Fixed in `55bbb27077`: `zeIKESAs` asks `show vpn ipsec sa | json` and `decodeIKESAs` decodes the records. Covered by `TestIKESAAnswerDecodesJSONAndRefusesTheTable` and `TestNATVerdictNeedsEveryFieldTrueOnOneSA`.
- **A latent selector-confusion in the same helper.** `assertZeSelectors` matched `ts-local` anywhere in the answer and `ts-remote` anywhere in the answer, so a local half from one Child SA and a remote half from another described a tunnel that does not exist. Both halves now come from ONE record.
- **`OriginalTSiAddr` and `OriginalTSrAddr` were written and read by nothing.** Found at closure. Fixed by reporting both in the SA payload, covered by `TestShowIPsecSAReportsThePreSubstitutionSelectorAddresses` and by `test/ipsec/ipsec-sa-show.ci`.

### Documentation Updates
- `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md`, `ipsec-14-responder.md`, `ipsec-8-ikev2-child-xfrm.md`, `ipsec-7-ikev2-engine.md`, `ipsec-10-cli-diag.md`, `ipsec-13-rekey-wire.md` -- the substitution, its gate, the per-side NAT fields and the payload, each with a `<!-- source: -->` anchor.
- `docs/guide/ipsec.md` -- a "Transport mode behind a NAT" section, extended at closure with the two original-selector fields.
- `docs/architecture/testing/interop.md` -- "The IPsec NAT box". Committed in `2e5da6d394`. A SECOND edit, the rule that a checker asserts on the structured answer, is written in the working tree and is NOT committed: that file's copy carries three other sessions' hunks, so naming it would commit their work. Recorded in Work Not Done.
- `docs/features.md` and `rfc/short/rfc7296.md` -- the NAT-traversal row and the Section 2.23.1 coverage sentence.
- `./le repository check` reports no stale or dangling source anchor in any file this spec touched (37 findings, all in other packages).

### Deviations from Plan
- The QEMU runner (AC-11, AC-12) and `test/ipsec/ipsec-transport-nat-selectors.ci` were not built. Both are homed in `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md`.
- `docs/architecture/testing/qemu-integration.md` was NOT edited, correctly: there is no action to list.
- `docs/guide/command-reference.md`, `docs/architecture/api/commands.md` and `docs/comparison.md` were answered Yes in the design-time checklist and needed no edit: none of them enumerates the SA payload's fields or carries a transport-mode row. The greps are in Documentation Verified.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The scenarios' assertions were written against the command's TEXT rendering | That rendering is a table whose column order follows the field names, so adding one field re-sorted it and three unrelated scenarios went red | Six scenarios failed on their first run, and the failure message carried the values it said were missing | The checkers read the `json` rendering; the rule is on `docs/architecture/testing/interop.md` and the row is in `plan/journal/green-that-could-not-have-been-red.md` |
| approach | The implementation commit named `internal/component/ike/engine/fsm.go`, `internal/le/interoplab/ipsec/checkers.go` and `test/interop-ipsec/parity_test.go` without diffing them first | All three carried another session's uncommitted EAP-TLS revocation work | Building the lab from a tree extracted at HEAD, and reading `git ls-tree` for the scenario directory the parity list names | Rows in `plan/journal/concurrent-session-corruption.md`; the habit is `git diff HEAD -- <path>` on every path a commit script names |
| escalation | A field was added to the SA and written at both roles with no reader outside tests | `ai/rules/completion.md` requires a non-test caller, and the operator had no way to see the pre-substitution pair the guide told them differs | Closure wiring pass | `original-tsi` and `original-tsr` in `saToMap` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Substitute on the RESPONDER before the SPD lookup | Done | `ts_nat_substitute.go::substituteResponderSelectors`, called from `ts_narrow.go::narrowChildSelectors` | One entry point covers `buildAuthResponse`, `startResponderEAP` and `respondChildRekey` |
| Substitute on the INITIATOR before any check | Done | `ts_nat_substitute.go::substituteInitiatorSelectors`, called from `ts_narrow.go::recordInitiatorSelectors` | Covers `adoptAuthResponseNegotiation` and `applyChildRekeyResponse` |
| Record which side is behind the NAT | Done | `sa.go::SA.PeerBehindNAT`, `responder.go::detectResponderNAT`, `fsm.go::handleSAInitResponse` | All four branches |
| Store the originals before substituting | Done | `ts_nat_substitute.go::storeOriginalSelectorAddresses` | Read by `show_ipsec.go::saToMap` |
| The substituted address is one this node OBSERVED | Done | `ts_nat_substitute.go::observedLocalAddress`, `observedRemoteAddress` | `peerEndpoint`'s only writer is `sa.go::adoptAuthenticatedEndpoint`, called in `responder.go` after `verifyRemoteAuth` and in `fsm.go` only once the SA reached `StateEstablished` |
| Prove it against a conforming peer across a real NAT | Done | `checkers.go::checkRealNATTransport`, `checkRealNATTunnelControl` | Green on 2026-09-06, red with the engine reverted |
| Prove it on Ze's runtime kernel | Not done | - | `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestResponderSubstitutesTransportSelectorsBeforeNarrowing`, `TestResponderAnswersTheIKESAAddressesBehindANAT` | The answer on the wire is read back through `pairsToWire` |
| AC-2 | Done | `TestResponderSubstitutesTransportSelectorsBeforeNarrowing` | The `BehindNAT` arm |
| AC-3 | Done | `TestInitiatorAdoptsSubstitutedTransportSelectorsBehindNAT`, `TestTransportSelectorsMatchTheIKESAAddressesBehindANAT` | No TS_UNACCEPTABLE: `recordInitiatorSelectors` returns nil |
| AC-4 | Done | `TestInitiatorAdoptsSubstitutedTransportSelectorsBehindNAT` | |
| AC-5 | Done | `TestSubstitutionStoresTheOriginalSelectorAddresses`, `TestShowIPsecSAReportsThePreSubstitutionSelectorAddresses` | The second reaches them through the operator entry point |
| AC-6 | Done | `TestSubstitutionStillRefusesAWidenedAnswer` | Three refusals and one acceptance, so it is not a blanket refusal |
| AC-7 | Done | `TestTunnelModeSelectorsUnchangedWithNATDetected`, `real-nat-tunnel-control` | Both arms of the gate |
| AC-8 | Done | `TestChildRekeyFloorComparedInSubstitutedSpace` | |
| AC-9 | Done | The RED block in "Progress, 2026-09-06, second session" | Engine reverted, Ze image rebuilt, fixed checker in place |
| AC-10 | Done | The verdict table in the same section | All three green, each ending in `verifyESPDirectionsToward` over `natESPDirections` |
| AC-11 | Not done | - | `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md` |
| AC-12 | Not done | - | Same spec |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestNATDetectionRecordsWhichSideIsBehindTheNAT` | Done | `internal/component/ike/engine/nat_detect_test.go` | Both roles, all four branches |
| `TestTransportProposalUsesObservedIKEAddresses` | Done | `internal/component/ike/engine/ts_nat_substitute_test.go` | |
| `TestResponderSubstitutesTransportSelectorsBeforeNarrowing` | Done | `ts_nat_substitute_test.go` | |
| `TestInitiatorAdoptsSubstitutedTransportSelectorsBehindNAT` | Done | `ts_nat_substitute_test.go` | |
| `TestSubstitutionStoresTheOriginalSelectorAddresses` | Done | `ts_nat_substitute_test.go` | |
| `TestSubstitutionStillRefusesAWidenedAnswer` | Changed | `ts_nat_substitute_test.go` | Landed beside the others rather than in `ts_initiator_subset_test.go` |
| `TestTunnelModeSelectorsUnchangedWithNATDetected` | Done | `ts_nat_substitute_test.go` | |
| `TestChildRekeyFloorComparedInSubstitutedSpace` | Changed | `ts_nat_substitute_test.go` | Same file rather than `child_rekey_initiator_answer_test.go` |
| `TestIPsecScenarioParity` | Done | `test/interop-ipsec/parity_test.go` | The three directories and their `nat.conf` requirement |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `sa.go`, `fsm.go`, `responder.go`, `rekey.go`, `transport_mode.go`, `ts_narrow.go`, `show_ipsec.go` | Done | |
| `ts_nat_substitute.go`, `ts_nat_substitute_test.go`, `nat_detect_test.go` | Done | |
| `internal/le/interoplab/ipsec/ipsec.go`, `checkers.go`, `helpers.go` | Done | The NAT plumbing landed in a new `nat.go` rather than in `ipsec.go` |
| `test/interop-ipsec/Dockerfile.nat` and the three scenario directories | Done | |
| `internal/le/qemu/actions.go`, `alltests.go`, `ipsec_nat_linux.go`, `gokrazy/kernel/runtime.require` | Not done | Owned by the new spec |
| `test/ipsec/ipsec-transport-nat-selectors.ci` | Not done | Owned by the new spec |
| `docs/architecture/testing/qemu-integration.md` | Changed | Correctly not edited: there is no action to list |
| Every other documentation page named | Done | See Documentation Updates |

### Audit Summary
- **Total items:** 12 acceptance criteria, 9 planned tests, 22 planned files
- **Done:** 10 of 12 AC, 7 of 9 tests as planned, 18 of 22 files
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 tests moved file, 1 file split into `nat.go`, 1 page correctly left alone, 4 files owned by a new spec

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| G-1 Ze establishes transport mode across a real NAT as INITIATOR and carries traffic | interop | `real-nat-transport-ze-initiator` PASS on 2026-09-06; `checkRealNATTransport` ends in `verifyESPDirectionsToward` over all four simplex directions, and `assertESPAdvanced` refuses unless a surviving SPI's XFRM byte counter GREW across a lossless ping |
| G-2 The same on the RESPONDER role | interop | `real-nat-transport-ze-responder` PASS, same assertions, role set by the fixture pair alone |
| G-3 The narrowing guard still refuses a genuinely wider answer | unit over the production adoption path | `TestSubstitutionStillRefusesAWidenedAnswer`: a responder-chosen TSr draws `errTSWidened` and `adoptAuthResponseNegotiation` answers `NotifyTSUnacceptable`; an any-port answer to a single-port proposal is refused; the conforming answer is accepted |
| G-4 Tunnel mode across the same NAT is unchanged | interop and unit | `real-nat-tunnel-control` PASS, asserting the INNER selectors `10.10.0.1/32` and `10.20.0.1/32` that no translation moves, plus `mode tunnel` on the installed state; `TestTunnelModeSelectorsUnchangedWithNATDetected` covers both arms of the gate |
| G-5 The proof is red-then-green against strongSwan behind a netfilter NAT, in Docker and in QEMU | interop, Docker half only | RED recorded in "Progress, 2026-09-06, second session": both transport scenarios fail at `wait for strongSwan SA ze timed out` with the engine reverted and the image rebuilt, so the Child SA never establishes without the substitution. The QEMU half is NOT met and is owned by `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| AC-11 and AC-12: the QEMU three-namespace runner, its registration, its workflow caller, and the `runtime.require` kernel symbols | The runner does not exist, and building it is a package of its own rather than a line in this closure | `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md` |
| `test/ipsec/ipsec-transport-nat-selectors.ci` | A `.ci` cannot reach a translation without namespaces, so it belongs with the runner | `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md` |
| The `docs/architecture/testing/interop.md` hunk stating that a checker asserts on the structured answer | Written in the working tree, not committable: that file carries three other sessions' uncommitted hunks, so naming it in a commit script would commit their work | `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md`, as a documentation row; the text is already written in the working tree |
| OQ-1, the Section 2.23.1 tunnel-mode fallback MAY | `ai/rules/rfc-compliance.md` reserves a MAY for Thomas, and this session cannot ask him. Ze answers TS_UNACCEPTABLE, which is conformant, so nothing is left non-conformant by the delay | Recorded in the Known Limitations of `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md` so the question survives this spec's deletion |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ipsec-transport-nat-selector-substitution-zeclose-ipsecnat-1151767.md`, 19 files, verdict clean |
| `./le spec session review check` | clean |
| Rounds | 1 |
| Reviewer lenses used | wiring and dead-symbol; logic and guard audit; security and untrusted input; documentation drift BEYOND the diff; RFC 7296 Section 2.23.1 conformance and its discrimination records; removed-behavior audit; simplicity and the Go style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `SA.OriginalTSiAddr` and `SA.OriginalTSrAddr` are written at both roles and read by nothing outside tests, so AC-5's "readable after the Child SA installs" was true only of a unit test. `docs/guide/ipsec.md` already told the operator that the wire pair and the programmed pair differ behind a NAT, and gave no way to see the first | `internal/component/ike/engine/ts_nat_substitute.go::storeOriginalSelectorAddresses`, `internal/component/ike/cmd/show_ipsec.go::saToMap` | `original-tsi` and `original-tsr` in the SA payload through `selectorAddressText`, with `TestShowIPsecSAReportsThePreSubstitutionSelectorAddresses` and two assertions in `test/ipsec/ipsec-sa-show.ci`. Observed RED with the two payload keys removed and the suite rebuilt (`output missing "original-tsi":null`), GREEN with them |
| 2 | BLOCKER, recorded and NOT repaired | `2e5da6d394` carried a third session's EAP-TLS revocation work in three files, not the one already journalled: the `cfg.CRLPEM = ca.CRLPEM()` hunk in `fsm.go`, `checkResponderEAPTLS13RevokedClient` with its `scenarioCheckers` entry in `checkers.go`, and the `responder-eap-tls13-revoked-client` row in `parity_test.go`. `(*CACertEntry).CRLPEM` and the scenario's eleven fixture files are all UNCOMMITTED, so at HEAD `internal/component/ike/engine` does not compile AND a registered scenario has no inputs | `internal/component/ike/engine/fsm.go`, `internal/le/interoplab/ipsec/checkers.go`, `test/interop-ipsec/parity_test.go` | Not repaired, by owner instruction and by `ai/rules/never-destroy-work.md`: removing any of the three deletes the other session's wiring, and committing their `store.go` and fixtures is the cross-commit `ai/rules/git-safety.md` forbids. Their commit repairs all three at once. Two rows are written into `plan/journal/concurrent-session-corruption.md` and that file is NOT in this closure's commit: `./le journal validate` refuses it over two OTHER sessions' rows whose Spec cell reads `crash-capture / vpp-isolated-cpus` and `crash-capture / firewall-domain-group`, which name no parseable stem, and naming the file would carry five foreign rows under this subject. This spec's committed journal row is the one `55bbb27077` landed in `plan/journal/green-that-could-not-have-been-red.md`; the finding itself is preserved by this table |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/ts_nat_substitute.go` | Yes | read in full at closure; 195 lines in `2e5da6d394 --stat` |
| `internal/component/ike/engine/ts_nat_substitute_test.go` | Yes | `gopls symbols` lists 8 test functions and 5 fixture helpers |
| `internal/component/ike/engine/nat_detect_test.go` | Yes | `gopls symbols` lists `TestNATDetectionRecordsWhichSideIsBehindTheNAT` and its 5 helpers |
| `internal/le/interoplab/ipsec/nat.go` | Yes | read in full at closure |
| `test/interop-ipsec/Dockerfile.nat` | Yes | 22 lines in `2e5da6d394 --stat` |
| `test/interop-ipsec/scenarios/real-nat-transport-ze-initiator/{ze,swanctl,nat}.conf` | Yes | all three read at closure; `nat.conf` declares `172.28.0.2 172.28.0.6` and `172.28.0.3 172.28.0.7` |
| `test/interop-ipsec/scenarios/real-nat-transport-ze-responder/`, `real-nat-tunnel-control/` | Yes | named in `ipsecScenarios`, which `TestEveryNativeScenarioHasCompleteInputs` walks; `go test ./test/interop-ipsec/` returns ok |
| `internal/le/qemu/ipsec_nat_linux.go`, `test/ipsec/ipsec-transport-nat-selectors.ci` | No | Work Not Done; owned by `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-8 | The substitution, its gate, its ceiling and its rekey floor | `./le job run label ipsec-close-unit command go test ./internal/component/ike/engine/ ./internal/component/ike/cmd/ ./internal/le/interoplab/ipsec/ ./test/interop-ipsec/` returned exit 0, `ok .../ike/engine 33.392s` |
| AC-5 | The originals are readable after the Child SA installs | `./le functional ipsec` -> `pass 19/19`, `20/24 PASS ipsec-sa-show`, which asserts `"original-tsi":null` and `"original-tsr":null` through the json rendering against a live daemon pair |
| AC-6 | The ceiling still refuses a widened answer | `TestSubstitutionStillRefusesAWidenedAnswer` passes in the run above; read at source it asserts `errTSWidened`, `NotifyTSUnacceptable` from `adoptAuthResponseNegotiation`, a port refusal, and one acceptance |
| AC-9 | The RED was observed with the image rebuilt and the FIXED checker in place | The pasted block in "Progress, 2026-09-06, second session". Its control failure text, `reports no IKE SA with nat-detected, behind-nat, peer-behind-nat all true`, is produced only by `assertNATVerdict` as `55bbb27077` rewrote it, which is what proves the fixed checker was the one that ran |
| AC-10 | All three scenarios green, each asserting the byte counter | The verdict table in the same section; `checkRealNATTransport` and `checkRealNATTunnelControl` both end in `verifyESPDirectionsToward(..., natESPDirections)`, and `natESPDirections` names all four simplex SAs |
| AC-11, AC-12 | - | Not done; Work Not Done |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `mode transport` peer answered from behind a NAT | none; `real-nat-transport-ze-initiator` | Yes: `checkRealNATTransport` establishes, waits for strongSwan's Child selectors, asserts Ze's NAT verdict and substituted selectors, requires `mode transport` on the INBOUND XFRM state, then pings and requires all four ESP counters to grow |
| `connection-type respond` peer dialled from behind a NAT | none; `real-nat-transport-ze-responder` | Yes: the same body, role set by the fixtures alone |
| NAT_DETECTION mismatch on either role | none; unit | Yes: `TestNATDetectionRecordsWhichSideIsBehindTheNAT` drives `detectResponderNAT` and `handleSAInitResponse` with real hashes, four cases per role |
| `show vpn ipsec sa` reports the SA facts | `test/ipsec/ipsec-sa-show.ci` | Yes: read at source and run; it asserts the payload keys through the json pipe against two live daemons |
| `./le qemu ipsec-nat-transport-test` | none | No: the action does not exist. Work Not Done |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | The three scenarios pass against strongSwan with no `encap = yes`, which is reachable only if strongSwan answers the substituted pair; `checkRealNATTransport` waits for exactly `172.28.0.3/32` and `172.28.0.6/32` on the strongSwan side |
| A-2 | confirmed, with its boundary recorded | `observedLocalAddress` returns the configured `local-address`, which `ikeListenHost` binds. The wildcard-bind case stays out of scope and belongs to `plan/immediate/spec-rfcgate-1b-rfc7296-pilot-deferred-ike-source-address.md`, as the assumption itself stated |
| A-3 | confirmed | No dataplane change was made, and all four XFRM byte counters advance across the translated path in each of the three scenarios |
| A-4 | confirmed | One netfilter box with a secondary address per peer makes all four comparisons mismatch: `assertNATVerdict` requires `nat-detected`, `behind-nat` and `peer-behind-nat` all true on ONE SA record, and it passes |
| A-5 | confirmed | The topology carries the scenarios' own ESP, and `natSetupScript` disables ICMP redirects so no peer learns to bypass the box |
| A-6 | broken as a claim, because it was never put to the test | The QEMU runner was never built, so the Alpine `apk` half and the runtime-kernel half of this assumption are unmeasured. Restated as G-2 of `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md`, whose AC-2 is the measurement |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #1 feature list | `docs/features.md` "IPsec NAT Traversal" states transport mode works across an address-translating NAT on both roles and anchors `ts_nat_substitute.go` | Yes |
| #3 CLI reference | `grep -rn "vpn ipsec" docs/guide/command-reference.md` returns only `clear vpn ipsec sa`; the page enumerates no SA payload field, so nothing there went stale | Yes, no edit owed |
| #4 API/RPC docs | `grep -rniE "ipsec" docs/architecture/api/commands.md` returns nothing | Yes, no edit owed |
| #6 user guide | `docs/guide/ipsec.md` "Transport mode behind a NAT", with both source anchors, extended at closure with the two original-selector fields | Yes |
| #9 RFC behavior | `rfc/short/rfc7296.md` Section 2.23.1 coverage sentence names both producers; `rfc/requirements/rfc7296.md` rows -1, -2 and -3 gained the NAT tests beside the pre-existing ones; six discrimination records in `rfc/discrimination/rfc7296.json`, and `./le rfc check` reports none of them stale | Yes |
| #10 test infrastructure | `docs/architecture/testing/interop.md` "The IPsec NAT box" is committed. The second hunk is uncommitted and is a Work Not Done row | Partial, recorded |
| #11 comparison table | `grep -rniE "ipsec\|transport mode" docs/comparison.md` returns nothing; the page carries no IPsec row | Yes, no edit owed |
| #12 internal architecture | Five `docs/architecture/ike/` pages edited, each with a `<!-- source: -->` anchor; `./le repository check` reports no stale anchor in any file this spec touched | Yes |
| Payload page | `docs/architecture/ike/ipsec-10-cli-diag.md` states both selector pairs and why the originals answer null rather than an empty string | Yes |
| Inventory drift found and NOT owned here | `docs/features.md` "IPsec Interop Testing" enumerates scenarios by hand and already omitted the `natt-*` pair, `delete-while-window-held` and `peer-reload-narrowing` before this spec. A hand list beside a registry (`ai/rules/evidence.md`); pre-existing, and the goal does not depend on it | NOTE |

## Core Insight

The defect and the first fix are the same mistake at two layers. The engine pinned
transport-mode selectors to the CONFIGURED address pair while its own comment claimed it
read the IKE SA, and the checkers asserted on the RENDERED table while claiming to read
what the command reported. Both were true by accident on the fixture in front of them: with
no NAT the configured and observed addresses are equal, and with `child-sa` first in the
column order the line-anchored regex matches. Both went false the moment a real variable
moved. A claim that holds only because two things coincide is not evidence, and the test
for it is the one this spec had to run twice: change the thing that makes them coincide,
and see whether the assertion still discriminates.

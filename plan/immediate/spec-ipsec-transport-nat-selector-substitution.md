# Spec: ipsec-transport-nat-selector-substitution

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-08-30 |

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
| A-4 | A single netfilter container doing DNAT plus MASQUERADE reproduces the RFC's two-NAT figure well enough to fire all four NAT_DETECTION comparisons | RFC 7296 Section 2.23.1 describes NAT A and NAT B as two boxes for exposition; the hashes compare addresses, not box counts | Only one substitution arm is exercised and the other ships unproven | The checker asserts `nat-detected` on Ze's `show vpn ipsec sa` and asserts the negotiated selectors carry the post-NAT addresses on both roles | unvalidated |
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

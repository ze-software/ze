# Spec: home-cpe-22-ike-responder-reauth

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P3 |
| Release reason | An `initiate` peer drops a reauthentication started by the other side (RFC 7296 Section 2.8.3 lets either party create the new IKE SA). The tunnel then breaks until the old SA is deleted and Ze reconnects. Interop behaviour with strongSwan gateways that schedule reauthentication; low impact because Ze reconnects within the backoff. |
| RFC gaps | - |
| Depends | - |
| Phase | 2/2 |
| Handoff | - |
| Updated | 2026-11-09 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

When the gateway, as responder of the original IKE SA, reauthenticates by sending
a fresh IKE_SA_INIT to the CPE, Ze drops it: `matchResponderPeer` considers only
`respond` peers. The gateway's reauthentication fails, it deletes the old IKE SA
(or lets it expire), and Ze's initiator loop reconnects from scratch: an outage
of one backoff step plus a full handshake, every reauthentication interval.

Accept, on an `initiate` peer that holds an ESTABLISHED IKE SA, a new
IKE_SA_INIT from that SA's authenticated peer address; authenticate it with the
same peer configuration; on success make it the peer's owner SA with its Child
SA, then retire the old SA, make before break. With no established SA, an
`initiate` peer still refuses unsolicited initiation.

The same supersede path serves a `respond` peer's re-initiation today, and it
supersedes on authentication alone: a parallel SA whose Child SA was refused
still takes over and the working Child SA is removed (owned journal row
`replacement-written-before-it-is-validated.md` 2026-11-02). The rule in
"Reauthentication outcomes" (supersede only once the successor's Child SA is
installed) is one rule for both connection types, so this spec fixes the
`respond` case too (AC-15, AC-16).

### Placement decision

This stays a child of its own rather than moving into
`plan/post-release/features/spec-ike-reauth.md`:

| Question | Answer |
|----------|--------|
| What does spec-ike-reauth own? | Ze SCHEDULING its own reauthentication (a reauth timer, a new config leaf, make-before-break as initiator). Its Task, Data Flow and ACs never mention a reauthentication the other side starts (read 2026-10-31) |
| What does this spec own? | RECEIVING one on an `initiate` peer: no config, no timer, a dispatch and ownership change |
| Why not merge? | One is a P3 post-release feature, the other a fix to how a shipped peer type answers a permitted peer action; `ai/META-DECISIONS.md` asks that fixes and features be split when release planning needs them apart |
| What do they share? | The adoption of a superseding IKE SA by a peer session. The `respond` side already has it (`ps.supersede`, `established.go`); this spec generalises it to `initiate` sessions, and spec-ike-reauth reuses it. The main thread adds a pointer line to spec-ike-reauth's Required Reading |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` - IKE door
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - responder path, accept-in-parallel
  → Constraint: a fresh IKE_SA_INIT never touches the established SA until the new SA authenticates (`tryResponderSAInit` comment, RFC 7296 Section 2.4).
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - peer session ownership
- [ ] `plan/post-release/features/spec-ike-reauth.md` - Status design, P3
  → Constraint: "reuse the full-handshake machinery with explicit old/new SA ownership."

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - present
  → Constraint: RFC 7296 Section 2.8.3: "Reauthentication is done by creating a new IKE SA from scratch (using IKE_SA_INIT/IKE_AUTH exchanges, without any REKEY_SA Notify payloads), creating new Child SAs within the new IKE SA (without REKEY_SA Notify payloads), and finally deleting the old IKE SA (which deletes the old Child SAs as well)."
  → Constraint: RFC 7296 Section 2.8.3: "While creation of a new IKE SA can be initiated by either party (initiator or responder in the original IKE SA), the use of EAP and/ or Configuration payloads means in practice that reauthentication has to be initiated by the same party as the original IKE SA."
  → Decision: a peer whose original SA used EAP or a Configuration payload (virtual IP) keeps refusing; the CPE uses X.509 and no CP.

**Key insights:**
- The new SA's role is responder while the peer's configured role is `initiate`; every role-dependent step (AUTH octets, rekey collision, DPD) must read the SA's role, not the peer's connection type.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/register.go` `matchResponderPeer` - skips every peer whose `ConnectionType` is not `respond`; compares the configured remote string to the datagram source.
- [ ] `register.go` `tryResponderSAInit` - cookie gate, one half-open handshake per peer (`responderBusy`), parallel acceptance while a tunnel is up, superseding only after the new SA authenticates.
- [ ] `internal/component/ike/engine/established.go` `maintainSA`, the `ps.supersede` arm - relinquish the old SA after the parallel SA authenticated, remove only the old Child SA (make before break), let `runResponder` promote the new one.
- [ ] `internal/component/ike/engine/responder.go` `finishResponderEstablish` - signals supersede (`signalSupersede`, `reconcile.go`) for every parallel SA that reaches IKE_AUTH, INCLUDING one whose Child SA was refused: `buildChildlessAuthResponse` establishes it with a nil Child SA. For a `respond` peer this happens today: the `maintainSA` supersede arm runs `cleanupChild` on the old Child SA, and `fsm.go` `resolvePendingAfterOwnerLoop` promotes the parallel SA, moving its Child SA only when one exists, so a childless successor takes ownership from a working tunnel (read 2026-11-02). This spec changes that for both connection types (Reauthentication outcomes).
- [ ] `internal/component/ike/engine/established.go` (the `sa.IsInitiator && sa.IKEAuthChildless` arm and `ps.childCreate`) - only the IKE SA initiator asks for a Child SA on a childless IKE SA later; a responder serves the peer's request (`respondNewChild`). Owner decision Q-5 (the reestablish exit in `maintainSA`): a session that loses its live Child SA reconnects rather than staying up childless.
- [ ] `internal/component/ike/ipsec/types.go` `ConnectionInitiate`, `ConnectionRespond` - connection types `initiate` and `respond` only.
- [ ] `test/interop-ipsec/scenarios/responder-accepts-reinit/` - strongSwan initiates against a Ze `respond` peer; nothing covers a reinit toward an `initiate` peer. `mobike-initiator/swanctl.conf` sets `reauth_time = 0s` on strongSwan.

**Behavior to preserve:**
- `initiate` peers refuse unsolicited initiation while no IKE SA is established.
- The cookie gate and the one-half-open rule.
- `respond` peers' matching, admission and supersede after a Child SA is installed unchanged.

**Behavior to change:**
- An `initiate` peer with an established SA accepts and adopts a reauthentication from that SA's peer address.
- A re-initiated SA, on either connection type, supersedes only once its Child SA is installed.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Responder peer match | `internal/component/ike/engine/register.go` `matchResponderPeer` | Returns a session only for a `respond` peer whose configured remote string equals the datagram source | extends: also returns an `initiate` session that holds an ESTABLISHED SA, for that SA's authenticated peer address |
| Responder IKE_SA_INIT gate | `internal/component/ike/engine/register.go` `tryResponderSAInit`, `responderBusy` | Cookie gate, one half-open handshake per peer, parallel acceptance while a tunnel is up | extends: the same gate serves the `initiate` session; no second admission path |
| Superseding SA adoption | `internal/component/ike/engine/established.go` `maintainSA`, the `ps.supersede` arm | After a parallel SA authenticates, relinquishes the old SA and removes only the old Child SA (make before break); `runResponder` promotes the new one | extends: generalised from `respond` sessions to `initiate` sessions; spec-ike-reauth reuses it |
| Peer Delete of the IKE SA | `internal/component/ike/engine/delete.go` (`peerDeletedIKE`), `inbound.go` `handleInformationalOwned`; `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` "A rekey the peer refuses" | A peer Delete of the owner IKE SA ends the session with `errSADeletedByPeer`, and `PeerSession.run` reconnects | extends: a Delete of the retired SA after adoption does not reconnect, the way a Delete of the old SA after a rekey already does not |
| Connection types | `internal/component/ike/ipsec/types.go` `ConnectionInitiate`, `ConnectionRespond` | Two types only | does not apply: no new type and no config leaf |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire: IKE_SA_INIT request with a zero responder SPI from the address of an established `initiate` peer's SA.

### Transformation Path
1. `tryResponderSAInit` asks the matcher; the matcher also returns an `initiate` session whose established SA's authenticated peer endpoint equals the source.
2. Cookie gate and busy CAS as today.
3. `newResponderSA` with the session's configuration; IKE_AUTH verifies the same identities and trust anchors.
4. On authentication the successor's Child SA (no REKEY_SA) is negotiated and installed into `pendingChild`. The supersede signal fires only once that Child SA is installed; the owner loop then retires the old SA and adopts the new one, so the new Child SA is installed before the old one is removed.
5. The gateway deletes the old IKE SA; Ze's delete handling finds it already retired and does not reconnect.

### Reauthentication outcomes

| Case | Ze's behaviour | Basis |
|------|----------------|-------|
| Successor authenticates, Child SA negotiated and installed | Supersede: the old Child SA is removed after the new one is installed, the old IKE SA is retired (AC-2) | RFC 7296 Section 2.8.3 order: new IKE SA, new Child SAs, then delete the old |
| Successor authenticates, Child SA refused in IKE_AUTH (any refusal `buildChildlessAuthResponse` answers, such as TS_UNACCEPTABLE or NO_PROPOSAL_CHOSEN) | The successor is kept as a childless IKE SA held beside the owner, NOT adopted. The old IKE SA and its Child SA keep ownership and carry the traffic. The log names the peer, the refusal notify and the fix (AC-9) | RFC 7296 Section 2.21.2: "This failure does not automatically cause the IKE SA to be deleted." Make before break: nothing exists to switch to |
| Successor authenticates, Child SA negotiated but its install into the dataplane fails | As the row above, and Ze also deletes the peer's half of the pair it could not install (`sendDeleteESP`, as `initiatorFirstChildSA`'s refusals already do), so the peer holds no Child SA Ze does not (AC-10) | Same |
| The gateway later creates a Child SA on the held successor (CREATE_CHILD_SA, `respondNewChild`) | Installed, then supersede as in the first row (AC-11) | Ze is responder of that IKE SA; only the IKE SA initiator asks for a Child SA later (`ps.childCreate`), so Ze waits for the gateway |
| The gateway deletes the old IKE SA while the successor is still childless | Ze does not keep a childless session (owner decision Q-5): it removes the old Child SA as the Delete requires, sends a Delete for the held successor, and reconnects as initiator on the existing backoff (AC-12) | Q-5 |
| The gateway deletes the held successor | The successor is dropped; the old SA is unchanged | |
| Any row above on a `respond` peer's re-initiation | The same behaviour; where an `initiate` peer reconnects as initiator (AC-12), a `respond` session takes the same reestablish exit and then waits for the peer's next IKE_SA_INIT, as it does today after that exit (AC-15, AC-16) | One supersede path serves both connection types |
| IKE_AUTH fails | The successor is discarded; the old SA is untouched (AC-4) | RFC 7296 Section 2.4 coexist, then supersede on authentication |

### Simultaneous replacements

Two replacements of one session's IKE SA can both complete: the gateway's
reauthentication crossing Ze's own IKE SA rekey (CREATE_CHILD_SA), or crossing
Ze's reconnect after Ze declared the old SA dead while the gateway's IKE_SA_INIT
was already admitted. RFC 7296 Section 2.8.3 says "IKEv2 does not have any
special support for reauthentication.", and Section 2.8.2's lowest-nonce rule
covers two CREATE_CHILD_SA rekeys of one IKE SA, not a new IKE_SA_INIT crossing a
replacement. The RFC does not settle the winner, so interop with strongSwan
settles it (strongSwan 6.1.0 source, read 2026-11-02):

- `src/libcharon/sa/ikev2/tasks/ike_reauth_complete.c` `build_i` queues
  `delete_ike_sa_job` for the OLD IKE SA once its reauthentication completes,
  whatever other IKE SA exists, so strongSwan always keeps the successor it
  started.
- `src/libcharon/sa/ike_sa_manager.c` `check_uniqueness` returns without
  touching duplicates under `UNIQUE_NO` (swanctl's default `unique = no`), so
  strongSwan never deletes a duplicate Ze created.

| Rule | What Ze does |
|------|--------------|
| Winner | The successor the PEER started (its reauthentication) wins over any replacement Ze started for the same session. The winner is decided by who started each SA, never by completion order, so both orders converge |
| Loser cleanup | Ze sends an INFORMATIONAL Delete for the replacement it started; that SA's Child SA states and policies are removed with the selector check that keeps the winner's policies (`removeChildSAExcept`, as the make-before-break path does); the SA leaves the SA table; the SPD ledger (`spd_left.go`) holds no entry of it |
| Never | Ze never deletes the peer-started successor, and never leaves two IKE SAs owned by one session |
| A rekey that loses the race | If the gateway deletes the old IKE SA before Ze's rekey completes, the rekey fails with its SA gone and nothing remains to clean |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ strongSwan | IKE_SA_INIT, IKE_AUTH, INFORMATIONAL Delete | No |
| Engine ↔ kernel | Child SA install then removal | No |

### Integration Points
- `matchResponderPeer`, `tryResponderSAInit`, the `ps.supersede` path, the initiator run loop in `fsm.go`/`reconcile.go`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Same responder path and gates |
| No unintended coupling | Yes | |
| No duplicated functionality | Yes | Generalises the existing supersede |
| Zero-copy preserved where applicable | N-A | |
| Registration over hardcoding, outbound | N-A | |
| Registration over hardcoding, inbound | N-A | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | strongSwan, as responder of the original SA with `reauth_time` set, reauthenticates by initiating a new IKE SA toward the peer | strongSwan 6.1.0 `src/libcharon/sa/ike_sa.c` `reauth` (read 2026-11-02): as the original responder it refuses only when `ike_sa_can_reauthenticate` fails (EAP or a virtual IP, logging "initiator did not reauthenticate as requested"); otherwise it logs "reauthenticating IKE_SA" and queues the reauth task | The defect never fires with strongSwan; spec drops to documentation of Ze's refusal | Source read; confirmed by the first scenario run's strongSwan log | confirmed |
| A-2 | The initiator run loop can hand ownership to a responder-role SA | `respond` sessions do it | A larger change to the run loop | Phase 1 wiring test | confirmed |
| A-3 | Behind a port-translating NAT the gateway's IKE_SA_INIT reaches Ze only through the mapping the old SA's NAT-T traffic keeps open, on UDP 4500, and Ze's NAT-T listener accepts an IKE_SA_INIT there | NAT keepalives; the lab NAT box's three-column row translates the port and installs no DNAT, so only conntrack carries an unsolicited datagram back (`docs/architecture/testing/interop.md`, "The IPsec NAT box") | Reauth from the gateway cannot work behind a NAT; documented in Known Limitations and the guide, the scenario then asserts Ze's clean reconnect instead | `real-nat-port-translated-responder-reauths-ze-initiator` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Simultaneous replacement: the gateway's reauthentication crosses Ze's IKE SA rekey or Ze's reconnect | Two IKE SAs established for one peer | The rule in "Simultaneous replacements": the peer-started successor wins, Ze deletes the replacement it started with its Child SA states and policies; proven by the two concurrent tests in both completion orders |
| R-4 | A childless successor takes ownership from a working tunnel | Ping stops after a reauth whose Child SA was refused | "Reauthentication outcomes": supersede waits for the successor's Child SA install; fault-injection tests AC-9 to AC-12 |
| R-2 | A spoofed IKE_SA_INIT from the peer address holds the half-open slot | Busy-drop log | Existing cookie gate applies unchanged |
| R-3 | Role-dependent code reads the peer's connection type | Rekey collision or DPD misbehaves after adoption | Grep for `ConnectionType` reads in the established path; test rekey after adoption |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `initiate` peers: tunnel torn down at reauth, or two SAs |
| How is it reverted? | Single commit |
| Who else touches this path? | `spec-ike-reauth.md`, `responder-accepts-reinit` scenario, child 4 (resolved remote for `respond` matching) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_SA_INIT from an established `initiate` peer's address | → | `matchResponderPeer` → `tryResponderSAInit` → supersede | `TestInitiatePeerAdoptsPeerStartedReauth` |
| strongSwan responder with `reauth_time` | → | full stack | interop `responder-reauths-ze-initiator` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `initiate` peer established; new IKE_SA_INIT from that SA's peer address | Accepted on the responder path; cookie gate applies |
| AC-2 | AC-1, IKE_AUTH authenticates | New IKE SA and Child SA adopted; old Child SA removed only after the new one is installed; traffic continues (ping loss at most one packet) |
| AC-3 | AC-2, gateway deletes the old IKE SA | No reconnect is triggered |
| AC-4 | AC-1, IKE_AUTH fails | Old SA untouched; failure logged |
| AC-5 | `initiate` peer with no established SA | IKE_SA_INIT dropped as today |
| AC-6 | Original SA used EAP or a Configuration payload | Dropped, with a log quoting the Section 2.8.3 reason |
| AC-7 | Adopted SA later rekeyed, by either side | Rekey works with Ze as responder of the IKE SA |
| AC-8 | Source differs from the SA's peer address | Dropped |
| AC-9 | AC-1, IKE_AUTH authenticates, Child SA refused (TS_UNACCEPTABLE injected) | Successor held childless, not adopted; old IKE SA and Child SA still own the session and carry traffic; log names the peer, the notify and the fix |
| AC-10 | AC-1, IKE_AUTH authenticates, the dataplane install of the successor's Child SA fails (injected) | As AC-9; Ze sends a Delete for the ESP pair it could not install; the fake dataplane holds no state or policy of it |
| AC-11 | AC-9, the gateway then creates a Child SA on the successor | Installed, then the successor supersedes as in AC-2 |
| AC-12 | AC-9, the gateway deletes the old IKE SA | Old Child SA removed; Ze sends a Delete for the successor and reconnects as initiator on the existing backoff (owner decision Q-5) |
| AC-13 | The gateway's reauthentication and Ze's own IKE SA rekey both complete, in either order; likewise with Ze's reconnect | Exactly one IKE SA and one Child SA pair remain: the gateway-started successor. Ze sent one Delete, for the replacement it started; no state, policy or SPD-ledger entry of it remains |
| AC-14 | Ze behind a port-translating NAT, gateway reauthenticates | The reauth IKE_SA_INIT arrives on Ze's UDP 4500 through the NAT mapping, is adopted as in AC-2, and the new Child SA encapsulates both ways |
| AC-15 | `respond` peer established; the peer re-initiates, IKE_AUTH authenticates, Child SA refused (TS_UNACCEPTABLE injected) or its install fails (injected) | Successor held childless, not adopted; the old IKE SA and Child SA keep ownership and their state and policies stay installed in the fake dataplane; a failed install also sends the Delete of AC-10 |
| AC-16 | AC-15, the peer then creates a Child SA on the successor | Installed, then the successor supersedes as in AC-2 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs the CPE against a gateway that reauthenticates hourly | gateway IKE_SA_INIT → adopt → old deleted | `responder-reauths-ze-initiator` |
| 2 | Runs the CPE behind a home NAT that translates ports | gateway IKE_SA_INIT through the NAT mapping → adopt | `real-nat-port-translated-responder-reauths-ze-initiator` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInitiatePeerAdoptsPeerStartedReauth` | `internal/component/ike/engine/rfc7296_reauth_receive_test.go` | AC-1, AC-2 | written |
| `TestInitiatePeerIgnoresDeleteOfRetiredSA` | same | AC-3 | not written: AC-3 is proven by the `responder-reauths-ze-initiator` interop checker (Deviations) |
| `TestInitiatePeerReauthFailureKeepsOldSA` | same | AC-4 | written |
| `TestInitiatePeerRefusesUnsolicitedInit` | same | AC-5, AC-8 | written |
| `TestInitiatePeerRefusesReauthAfterEAPOrCP` | same | AC-6 | written |
| `TestAdoptedResponderRoleSARekeys` | `internal/component/ike/engine/rfc7296_reauth_receive_test.go` | AC-7, the Child SA half | written |
| `TestAdoptedResponderRoleSAIKERekeys` | `internal/component/ike/engine/reauth_crossing_test.go` | AC-7, the IKE SA half: an IKE SA adopted with Ze as its responder is rekeyed by either side (RFC 7296 Section 2.18) | written |
| `TestInitiatePeerReauthChildRefusedKeepsOldSA` | `internal/component/ike/engine/rfc7296_reauth_receive_test.go` | AC-9: successor's IKE_AUTH answered with TS_UNACCEPTABLE; old SA still owner, its Child SA still installed in the fake dataplane | written |
| `TestInitiatePeerReauthChildInstallFailureKeepsOldSA` | same | AC-10: fake dataplane fails the successor's state install; nothing of it left installed; the IKE_AUTH answer carries the refusal, so no ESP Delete is owed (Deviations) | written |
| `TestInitiatePeerHeldSuccessorAdoptedOnPeerChildCreate` | same | AC-11 | written |
| `TestInitiatePeerOldDeletedWhileSuccessorChildlessReconnects` | same | AC-12: Delete sent for the successor, session back in the initiator loop | written |
| `TestSimultaneousReauthAndIKERekeyKeepsPeerStarted` | `internal/component/ike/engine/reauth_crossing_test.go` | AC-13 with Ze's IKE SA rekey; runs both completion orders, under `-race` | written |
| `TestReconnectCrossingPeerReauthKeepsPeerStarted` | same | AC-13 with Ze's reconnect; runs both completion orders, under `-race` | written |
| `TestRespondPeerReinitChildRefusedKeepsOldSA` | `internal/component/ike/engine/rfc7296_reauth_receive_test.go` | AC-15, the TS_UNACCEPTABLE injection: old SA still owner, its Child SA still installed | written; red with the `responder.go` child-nil hold reverted (`c22-unit-broken2.log`) |
| `TestRespondPeerReinitChildInstallFailureKeepsOldSA` | same | AC-15, the install-failure injection | written |
| `TestRespondPeerHeldSuccessorAdoptedOnPeerChildCreate` | same | AC-16 | written |
| `TestHeldSuccessorAnswersLivenessAndDelete` | same | A held successor answers a liveness probe and the peer's Delete (tags RFC7296-1.4-4, RFC7296-1.4.1-6, records 81098d908d) | written |
| `TestSuccessorReadyNeedsBothSlots` | `internal/component/ike/engine/reauth_successor_ready_test.go` | `successorReady` answers true only with both the successor SA and its Child SA staged; added at closure because a break of that predicate left `TestRespondPeerReinitChildRefusedKeepsOldSA` green (`c22-unit-broken.log`) | written at closure, run owed by the main thread |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-respond-reinit-supersede` | drafted at `test/draft/ipsec/ipsec-respond-reinit-supersede.ci`, promotes to `test/ipsec/ipsec-respond-reinit-supersede.ci` | Two Ze daemons (`option=exclusive:group=ipsec-xfrm`; the `respond` peer runs the real XFRM backend, the `initiate` peer the noop one): the `initiate` daemon is killed and started again while the `respond` daemon still holds the old IKE SA, so its new IKE_SA_INIT is a parallel re-initiation on the `respond` peer; asserts on the `respond` peer's XFRM state only (no `show vpn ipsec sa` read) that the old Child SA's states stay until the successor's are installed, then leave, with the policies unchanged (AC-2 and AC-15 on the daemon path). The `initiate`-peer path needs a gateway that schedules reauthentication, which no Ze daemon does before `spec-ike-reauth.md`, so its daemon proof is the interop scenarios below | promoted to `test/ipsec/ipsec-respond-reinit-supersede.ci` (3feb1d4157); clean run passes and the run with `signalSupersede` removed from `finishResponderEstablish` fails on "the superseded Child SA's states stayed in the kernel" |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `responder-reauths-ze-initiator` | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0 responder, `reauth_time` short, `make_before_break = yes` | AC-1 to AC-3 and AC-7: new SPIs, continuous ping, no reconnect log | pass (`ix-full3.log`, 1m4s); red with the matcher reverted (`dockerhost-run.log`: "ze did not adopt the IKE SA strongSwan created to reauthenticate") |
| `real-nat-port-translated-responder-reauths-ze-initiator` | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0 responder outside the lab NAT box, `reauth_time` short, `make_before_break = yes`; `nat.conf` holds two rows: the three-column row for Ze (`172.28.0.2  172.28.0.6  14500`: Ze's UDP 4500 leaves the box from 172.28.0.6:14500, no DNAT) and a two-column row for strongSwan (`172.28.0.3  172.28.0.7`), because Ze initiates before a checker could route through the NAT, so Ze needs an address on the box to dial | AC-14 and A-3. Assertions: strongSwan logs "reauthenticating IKE_SA"; its reauth IKE_SA_INIT is sent to 172.28.0.6:14500 and Ze logs its adoption; the IKE SPIs are not read: the ESP SPIs changing on both sides plus Ze's adoption line stand in for them; Ze's new Child SA encapsulates to strongSwan's 4500 and strongSwan's to dport 14500; the ping across the tunnel loses at most one packet; Ze logs no reconnect. Red walk: the matcher change reverted and the image rebuilt, strongSwan's reauth fails and Ze's reconnect line appears | pass (`ix-full3.log`, 2m16s); red with the matcher reverted (`dockerhost-run.log`: "ze did not adopt the IKE SA strongSwan created through the NAT mapping") |

## Files to Modify
- `internal/component/ike/engine/register.go` - matcher also returns an established `initiate` session for its SA's peer address
- `internal/component/ike/engine/established.go`, `fsm.go`, `reconcile.go` - the initiator loop adopts a superseding SA
- `internal/component/ike/engine/delete.go` - a delete for the retired SA does not trigger reconnect
- `internal/component/ike/engine/responder.go` `finishResponderEstablish` - a parallel SA signals supersede only once its Child SA is installed; a childless one is held beside the owner (both connection types)
- `docs/guide/ipsec.md` - reauthentication by the gateway
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - declared by `established.go` and `delete.go`. "A rekey the peer refuses" says a peer Delete of the IKE SA ends the session with `errSADeletedByPeer` and reconnects; after this change a Delete of an SA retired by an adopted reauthentication does not, so that paragraph names the exception

## Files to Create
- `internal/component/ike/engine/reauth_receive_test.go`
- `test/interop-ipsec/scenarios/responder-reauths-ze-initiator/`
- `test/interop-ipsec/scenarios/real-nat-port-translated-responder-reauths-ze-initiator/` (`ze.conf`, `swanctl.conf`, `nat.conf`)

### Targeted invocations

| Proof | Command |
|-------|---------|
| Unit tests | `./le job run label unit-pkg command go test -race -run 'TestInitiatePeer|TestRespondPeer|TestAdoptedResponderRoleSARekeys|TestSimultaneousReauthAndIKERekeyKeepsPeerStarted|TestReconnectCrossingPeerReauthKeepsPeerStarted' ./internal/component/ike/engine/` |
| Interop, no NAT | `IPSEC_INTEROP_SCENARIO=responder-reauths-ze-initiator ./le test integration interop-ipsec` |
| Interop, real NAT | `IPSEC_INTEROP_SCENARIO=real-nat-port-translated-responder-reauths-ze-initiator ./le test integration interop-ipsec` |
| Regression | `IPSEC_INTEROP_SCENARIO=responder-accepts-reinit ./le test integration interop-ipsec` |

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |
| BGP family surface | N-A | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Feature scope changed? | Yes | the IPsec feature file under `features/` |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior changed? | Yes | `rfc/short/rfc7296.md` Section 2.8.3 row if one exists |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-14-responder.md` |
| 13 | Route metadata keys | No | |
| 14 | Prometheus counters | No | |
| 15 | Registered plugin/event changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-22-ike-responder-reauth.md` |
| 17 | Existing examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring and A-1 (MANDATORY FIRST)**
   - Tests: `TestInitiatePeerAdoptsPeerStartedReauth` (red); the interop scenario run once against today's Ze to observe strongSwan's reauth and Ze's drop
   - Verify: A-1 confirmed with strongSwan's log; if strongSwan never reinitiates, stop and report
2. **Phase: Adoption** - AC-1 to AC-16; scenarios green; the matcher change reverted once and recorded red; the supersede-waits-for-install change reverted once with `TestRespondPeerReinitChildRefusedKeepsOldSA` recorded red

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Role read from the SA, not the connection type, after adoption |
| Rule: make before break | New Child SA installed before the old is removed |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Scenario | `ls test/interop-ipsec/scenarios/responder-reauths-ze-initiator` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Identity | New SA authenticates the same remote identity and anchor as the old |
| DoS | Cookie gate and one half-open slot unchanged |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 broken | Report; spec reduces to a guide sentence, owner decides |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

`respond` and `initiate` describe who dials first, not who may create the next IKE
SA. The responder path already knew how to replace an SA; it only asked the wrong
question about which peers it serves.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `replacement-written-before-it-is-validated.md` | 2026-11-02 | `finishResponderEstablish` supersedes a working tunnel with a childless re-initiated SA (`respond` peers) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Stand-alone child, pointer from spec-ike-reauth | Fold into spec-ike-reauth | See Placement decision |
| Accept only while an SA is established, only from its authenticated peer address | Accept any initiation on `initiate` peers | Keeps `initiate` semantics; a reauth only exists relative to an SA |
| Refuse after EAP or CP | Accept always | RFC 7296 Section 2.8.3 says such reauth "has to be initiated by the same party as the original IKE SA" |

## Known Limitations
- Behind a NAT without an open mapping the gateway cannot reach the CPE (A-3).
- The CPE's own configuration (IKE lifetime 0) does not prevent the gateway from reauthenticating.

## RFC Documentation (Scope: protocol)

The matcher's new branch carries the RFC 7296 Section 2.8.3 "either party" quote;
the EAP/CP refusal carries the "same party" quote.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass: every command in "Targeted invocations" (the unit tests, the no-NAT and real-NAT interop scenarios, and the `responder-accepts-reinit` regression); no `./le verify` run, GitHub CI verifies the pushed commit
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

## Implementation Summary

### What Was Implemented
- `matchReauthenticatingPeer` (`internal/component/ike/engine/reauth_receive.go`): `tryResponderSAInit` (`register.go`) asks it after `matchResponderPeer`, so an `initiate` peer whose owner loop holds an IKE SA admits a fresh IKE_SA_INIT from that SA's published endpoint (`saEndpoint`), through the unchanged COOKIE gate and half-open slot; an EAP peer refuses with a WARN naming the fix (67700fd37e).
- `finishResponderEstablish` (`responder.go`) signals supersede only when the successor's Child SA is installed, and holds a childless successor beside the owner, for both connection types (67700fd37e).
- `adoptPeerStartedSuccessor`, `promotePendingSA`, `retireHeldSuccessor`, `forgetSecondSlot` (`reauth_receive.go`) and the `runInitiator` tail (`fsm.go`, `continueOnPeerStartedSuccessors`): the initiator run loop adopts a ready successor with Ze as its responder, or deletes a held childless one and reconnects (67700fd37e, fc9b7f5a3b).
- `handleHeldSuccessorInbound` (`held_successor.go`): a held successor answers retransmissions, a peer CREATE_CHILD_SA (then supersedes), a Child SA rekey with CHILD_SA_NOT_FOUND, an IKE rekey with TEMPORARY_FAILURE, a liveness probe and a Delete (f50e28c360).
- `recordCrossing`, `relinquishToSuccessor`, `successorReady` (`reauth_receive.go`) and the `maintainSA` supersede deferral (`established.go`): a reauthentication crossing Ze's own IKE SA rekey or reconnect keeps the peer-started successor and deletes Ze's replacement (fc9b7f5a3b).
- Interop scenarios `responder-reauths-ze-initiator` (fc9b7f5a3b) and `real-nat-port-translated-responder-reauths-ze-initiator` (9689481ddb), checkers in `internal/le/interoplab/ipsec/reauth.go`; functional test `test/ipsec/ipsec-respond-reinit-supersede.ci` (3feb1d4157).
- Closure: `TestSuccessorReadyNeedsBothSlots` (`reauth_successor_ready_test.go`), because no existing test went red when `successorReady` lost its Child SA half.

### Bugs Found/Fixed
- The owned journal row (`replacement-written-before-it-is-validated.md`, 2026-11-02): a `respond` peer's childless re-initiation superseded a working tunnel. Fixed in `finishResponderEstablish`; `TestRespondPeerReinitChildRefusedKeepsOldSA` goes red with the hold reverted (`c22-unit-broken2.log`). The row is marked `fixed:` in commit A.
- The initiator loop woke only on its retransmit timer after its handshake completed, so an established SA showed no Child SA for up to 500 ms (1332f86cc1, `handshake_wake_test.go`).

### Documentation Updates
- `docs/guide/ipsec.md` (reauthentication by the gateway, anchor `reauth_receive.go -- matchReauthenticatingPeer, adoptPeerStartedSuccessor, relinquishToSuccessor`).
- `docs/architecture/ike/ipsec-14-responder.md` (held successor and crossing paragraphs, anchors `reauth_receive.go`, `held_successor.go`).
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (the Delete exception after an adopted reauthentication, anchor `reauth_receive.go -- adoptPeerStartedSuccessor`).
- `features/ikev2-engine.md` (the reauthentication sentence with both anchors), `ai/digests/ipsec-ike.md`.
- `./le doc check all` at closure: the Source anchors and Digest anchors sections pass; it fails on command-surface drift (`clear debug` pipe-operator availability) and the YANG/handler contract, neither touching IKE.

### Deviations from Plan
- AC-3 has no unit test `TestInitiatePeerIgnoresDeleteOfRetiredSA`: the `responder-reauths-ze-initiator` checker (`checkResponderReauthsZeInitiator`) waits for strongSwan's reauthentication, which deletes the old IKE SA, and refuses Ze's reconnect line, so the full stack proves it.
- AC-10: no ESP Delete is sent, because none is owed. A failed install answers the IKE_AUTH with the childless refusal before the peer gets its half (`finishResponderEstablish` comment, `expectChildlessAuthAnswer`), so the peer never holds the pair. The test asserts that answer instead.
- The pointer from `plan/post-release/features/spec-ike-reauth.md` to this work was not added (Work Not Done).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The spec said a failed successor install makes Ze send a Delete for the ESP pair (AC-10) | The refusal rides in the IKE_AUTH answer, so the peer installs nothing and no Delete is owed | Implementation of `buildFinalAuthResponse`, recorded in the state file | AC-10 test asserts the childless answer; Deviations |
| approach | The supersede-waits unit red walk broke `successorReady` and the AC-15 test stayed green (`c22-unit-broken.log`) | `successorReady` had no test of its own | Main thread's red walk | `TestSuccessorReadyNeedsBothSlots` added at closure |
| escalation | The implementation commits carry `Spec-work: home-cpe-1-logical-interface-name` | The work is this spec's; the agent edited under the parent's claim when the WIP cap refused its own | `git log` at closure | Named in the commit A body |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Accept a reauth IKE_SA_INIT on an established `initiate` peer from its SA's peer address | Done | `reauth_receive.go` `matchReauthenticatingPeer`, called from `register.go` `tryResponderSAInit` | |
| Authenticate with the same peer configuration | Done | `register.go` `tryResponderSAInit` `newResponderSA(ps.peerName, ps.peerCfg, ...)` | |
| Adopt with its Child SA, then retire the old SA, make before break | Done | `reauth_receive.go` `adoptPeerStartedSuccessor`, `relinquishToSuccessor` | |
| No established SA: still refuse | Done | `matchReauthenticatingPeer` (`ownedSA` nil skips) | |
| `respond` re-initiation supersedes only once its Child SA is installed | Done | `responder.go` `finishResponderEstablish`, the `child == nil` hold | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestInitiatePeerAdoptsPeerStartedReauth`; interop `responder-reauths-ze-initiator` | |
| AC-2 | Done | `TestInitiatePeerAdoptsPeerStartedReauth`; real-NAT scenario ping loss at most one (`requireReauthPingLoss`) | |
| AC-3 | Changed | interop `responder-reauths-ze-initiator` (no reconnect line) | Deviations |
| AC-4 | Done | `TestInitiatePeerReauthFailureKeepsOldSA` | |
| AC-5 | Done | `TestInitiatePeerRefusesUnsolicitedInit` | |
| AC-6 | Done | `TestInitiatePeerRefusesReauthAfterEAPOrCP` | |
| AC-7 | Done | `TestAdoptedResponderRoleSARekeys`, `TestAdoptedResponderRoleSAIKERekeys` | |
| AC-8 | Done | `TestInitiatePeerRefusesUnsolicitedInit` | |
| AC-9 | Done | `TestInitiatePeerReauthChildRefusedKeepsOldSA` | |
| AC-10 | Changed | `TestInitiatePeerReauthChildInstallFailureKeepsOldSA` | no Delete owed, Deviations |
| AC-11 | Done | `TestInitiatePeerHeldSuccessorAdoptedOnPeerChildCreate` | |
| AC-12 | Done | `TestInitiatePeerOldDeletedWhileSuccessorChildlessReconnects` (`expectIKEDelete`) | |
| AC-13 | Done | `TestSimultaneousReauthAndIKERekeyKeepsPeerStarted`, `TestReconnectCrossingPeerReauthKeepsPeerStarted` | |
| AC-14 | Done | interop `real-nat-port-translated-responder-reauths-ze-initiator` | |
| AC-15 | Done | `TestRespondPeerReinitChildRefusedKeepsOldSA`, `TestRespondPeerReinitChildInstallFailureKeepsOldSA`; `ipsec-respond-reinit-supersede.ci` | |
| AC-16 | Done | `TestRespondPeerHeldSuccessorAdoptedOnPeerChildCreate` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every unit test of the TDD table except `TestInitiatePeerIgnoresDeleteOfRetiredSA` | Done | `rfc7296_reauth_receive_test.go`, `reauth_crossing_test.go` | file renamed by c78728cdf4 |
| `TestInitiatePeerIgnoresDeleteOfRetiredSA` | Changed | interop checker | Deviations |
| `ipsec-respond-reinit-supersede` | Done | `test/ipsec/ipsec-respond-reinit-supersede.ci` | clean and broken logs |
| `responder-reauths-ze-initiator`, `real-nat-port-translated-responder-reauths-ze-initiator` | Done | `test/interop-ipsec/scenarios/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `register.go`, `established.go`, `fsm.go`, `reconcile.go`, `responder.go` | Done | |
| `delete.go` | Changed | not edited: the retired SA is no longer the owned SA, so `routeInbound` never hands its Delete to the owner loop's reconnect exit |
| `docs/guide/ipsec.md`, `ipsec-8-ikev2-child-xfrm.md` | Done | |
| `reauth_receive_test.go` | Changed | now `rfc7296_reauth_receive_test.go` |
| Both scenario directories | Done | |

### Audit Summary
- **Total items:** 16 ACs, 5 requirements
- **Done:** 14 ACs, 5 requirements
- **Partial:** 0
- **Skipped:** 0
- **Changed:** AC-3, AC-10 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An `initiate` peer adopts a reauthentication its gateway starts, make before break, no reconnect | interop | `responder-reauths-ze-initiator` pass (`ix-full3.log`), red with the matcher reverted (`dockerhost-run.log`) |
| The same behind a port-translating NAT | interop | `real-nat-port-translated-responder-reauths-ze-initiator` pass (`ix-full3.log`), red with the matcher reverted |
| A re-initiated SA supersedes only once its Child SA is installed, both connection types | functional | `test/ipsec/ipsec-respond-reinit-supersede.ci` clean pass (`rwinv/out-gb6/ipsec-clean.log`, test 25), red with `signalSupersede` removed (`rwinv/out-gb6/c22-reinit-broken.log`, overlay `rwinv/home-cpe-22-reinit-supersede-ci`) |
| A childless successor never ends a working tunnel | unit | `TestRespondPeerReinitChildRefusedKeepsOldSA` red with the hold reverted (`c22-unit-broken2.log`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The Required Reading pointer from spec-ike-reauth to the supersede mechanism this spec generalised (`docs/architecture/ike/ipsec-14-responder.md`) | That spec has no `## Existing Mechanisms` section, so the spec tooling refuses an edit to it; it belongs to that spec's own design pass | `plan/post-release/features/spec-ike-reauth.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-22-ike-responder-reauth-7e292f27-ced0-427f-a7b6-3a931db211f9.md` |
| `./le spec review check` | clean |
| Rounds | 1 |
| Reviewer lenses used | wiring (`tryResponderSAInit` to `matchReauthenticatingPeer`, `runInitiator` to `adoptPeerStartedSuccessor`), security (identity from `ps.peerCfg`, COOKIE gate before the CAS, no peer-reachable panic in `held_successor.go`), logic of the held-successor answers, closure diff |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | No BLOCKER or ISSUE. NOTE: an unexpected exchange type on a held successor is dropped at Debug with no answer (`handleHeldSuccessorInbound` default arm). NOTE: `successorReady` had no direct test; added | | |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/rfc7296_reauth_receive_test.go`, `reauth_crossing_test.go`, `reauth_successor_ready_test.go` | Yes | `ls internal/component/ike/engine/*reauth*` |
| `test/interop-ipsec/scenarios/responder-reauths-ze-initiator/`, `real-nat-port-translated-responder-reauths-ze-initiator/` | Yes | `ls test/interop-ipsec/scenarios/` |
| `test/ipsec/ipsec-respond-reinit-supersede.ci` | Yes | promoted in 3feb1d4157 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-8 | Matcher wired after `matchResponderPeer` | `tryResponderSAInit` calls `matchReauthenticatingPeer(pkt.RemoteAddr, log)` when `matchResponderPeer` answers nil |
| AC-15 | Supersede only with a Child SA | `finishResponderEstablish` returns on `child == nil` before `signalSupersede` |
| AC-2, AC-15 | Daemon path | `ipsec-clean.log` test 25 PASS; `c22-reinit-broken.log` FAIL "the superseded Child SA's states stayed in the kernel" |
| all unit ACs | Unit tests pass | owed: the main thread runs the command in the report (the rename c78728cdf4 changed no content) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| IKE_SA_INIT on an established peer | `test/ipsec/ipsec-respond-reinit-supersede.ci` (respond path) | clean pass and broken fail |
| strongSwan responder with `reauth_time` | interop `responder-reauths-ze-initiator` | pass, matcher-revert red |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | the scenario's checker waits for Ze's "adopting the IKE SA the gateway created to reauthenticate" line, which only a strongSwan-started IKE SA produces; pass in `ix-full3.log` |
| A-2 | confirmed | `TestInitiatePeerAdoptsPeerStartedReauth`, `TestReconnectCrossingPeerReauthKeepsPeerStarted` drive the real run loop |
| A-3 | confirmed | `real-nat-port-translated-responder-reauths-ze-initiator` pass |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature, guide, architecture pages | anchors to `reauth_receive.go` and `held_successor.go` resolve (`./le doc check all`, Source anchors section) | Yes |
| RFC 7296 Section 2.8.3 row | `rfc/short/rfc7296.md` holds no Section 2.8.3 row: the section states no MUST, so no row exists to update | Yes |
| Config, CLI, API, wire, plugin | no YANG, command or wire change in the diff | Yes |

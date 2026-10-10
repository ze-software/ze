# Spec: ike-create-child-sa-on-established-ike-sa

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - (the interop proof runs on a Docker host that passes the Ze kernel check, the QEMU docker-lab route of `plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md`) |
| Phase | 1/5 |
| Handoff | - |
| Updated | 2026-10-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze never creates a NEW Child SA on an established IKE SA. Its CREATE_CHILD_SA
support is limited to rekeying the one Child SA that IKE_AUTH built, and rekeying
the IKE SA itself. Two consequences, both met by an operator facing strongSwan:

| Role | What happens today | What strongSwan does |
|------|--------------------|----------------------|
| Responder | The initiator authenticates but the piggybacked Child SA is refused (no ESP proposal, no acceptable selectors). Ze answers with the error notify ALONE (no IDr, no AUTH) and kills the IKE SA, so the initiator cannot even complete authentication | strongSwan by default keeps the IKE SA ("failed to establish CHILD_SA, keeping IKE_SA", `close_ike_on_child_failure` default FALSE) and later asks for the Child SA with CREATE_CHILD_SA, which Ze refuses with NO_PROPOSAL_CHOSEN |
| Initiator | The responder authenticates and refuses the Child SA (RFC 7296 Section 2.21.2). Since f0006faaa2 Ze logs the notify, sends an IKE Delete, and backs off (`deleteChildlessIKESA`, `errChildSARefused`). A response with neither SAr2 nor an error notify still establishes, then fails "no peer ESP SPI recorded" and sends no Delete (journal row, `plan/journal/silent-fall-through.md`, 2026-10-10) | strongSwan as responder keeps its half of the IKE SA and accepts a later CREATE_CHILD_SA for the child |

Goal: RFC 7296 Section 1.3.1 new Child SA creation in both roles, a childless IKE
SA that stays useful (DPD, rekey, Delete, show), and the Section 2.21.2 behavior:
the responder keeps the IKE SA when only the piggybacked Child SA fails, and the
initiator does not fail authentication and creates the Child SA later. Owner
decision 2026-10-10: Section 2.21.2 gets its own spec, this one.

Out of scope: more than one Child SA per IKE SA (Ze holds one, `PeerSession.childSA`);
RFC 6023 childless IKE SA INITIATION (CHILDLESS_IKEV2_SUPPORTED), which lets an
IKE_AUTH carry no SAi2 at all. Both are separate features; see Known Limitations.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - IKE_AUTH handling, the Section 2.21.2 paragraph among the NAT and refusal bullets
  → Constraint: the paragraph "RFC 7296 Section 2.21.2 lets a responder authenticate..." states "Ze cannot create a Child SA after IKE_AUTH" and describes `deleteChildlessIKESA`; this spec makes it false, so the paragraph is rewritten in the same phase that removes `deleteChildlessIKESA` (ai/rules/documentation.md).
  → Decision: the source anchors `delete.go -- deleteChildlessIKESA` and `fsm.go -- errChildSARefused` are removed with the symbols.
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - Child SA install, "A rekey the peer refuses" table
  → Constraint: the refusal table (TEMPORARY_FAILURE 60 s hold, NO_ADDITIONAL_SAS re-establish, INVALID_KE_PAYLOAD retry in a configured group, other error waits `rekeyRefusedWait`, one record per kind) is the contract the creation exchange must follow; creation gets its own rows, its own per-kind record and its own holds, so a refused creation never holds a Child or IKE rekey and the reverse.
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - responder IKE_AUTH and `finishResponderEstablish`, parallel re-init (Section 2.4) with `pendingSA`/`pendingChild`
  → Constraint: the supersede path promotes `pendingChild`; a nil pending child must promote cleanly (childless supersede), never reach `removeChildSAExcept` with nil.
- [ ] `docs/architecture/ike/ipsec-13-rekey-wire.md` - make-before-break rekey on the owner loop
  → Constraint: all post-establishment SA and Child SA mutation happens on the `maintainSA` goroutine; creation is started and completed there, through `ps.pendingRekey` and the single request window (RFC 7296 Section 2.3, window of one).
- [ ] `docs/architecture/ike/ipsec-10-cli-diag.md` - `PeerInfo` snapshot and `show vpn ipsec sa` (declared by `metrics.go`)
  → Constraint: `PeerInfo` describes the Child SA as INSTALLED, never as configured, and `child-sa` in `show vpn ipsec sa` carries the installed transform; a childless session therefore shows no `child-sa` content rather than the configured esp-group, and the page states what a childless peer shows.
- [ ] `docs/architecture/testing/interop.md` - IPsec interop suite
  → Constraint: scenarios live in `test/interop-ipsec/scenarios/<name>/` (swanctl.conf plus ze.conf), the checker is a `scenarioCheckers` entry in `internal/le/interoplab/ipsec/checkers.go`, the parity list is `test/interop-ipsec/parity_test.go`; names carry no numeric prefix; runs need a Docker kernel that passes the Ze kernel check.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - requirement rows and the Support coverage cell
  → Constraint: RFC7296-2.21.2-2 [MUST NOT] "the initiator MUST NOT fail the authentication because of this" is unticked; this spec adds its tagged proof on the initiator path.
  → Constraint: RFC7296-1.3.1-1 and RFC7296-1.3.1-2 (USE_TRANSPORT_MODE echo; delete when declined and unacceptable) apply to a created Child SA exactly as to the IKE_AUTH one: a creation request carries USE_TRANSPORT_MODE under `mode transport`, and `transport-required` deletes a tunnel-mode answer.
  → Constraint: RFC7296-1.3-1 (KEi group in an SA offer), RFC7296-1.3-2 (INVALID_KE_PAYLOAD), RFC7296-2.9-1 and -2 (narrowing), RFC7296-2.17-1 and -2 (KEYMAT order), RFC7296-2.21.3-1 (every errored request on an authenticated SA draws an error response) bind the creation exchange.
  → Decision: the Support coverage cell gains a Sections 1.3.1 and 2.21.2 sentence naming the producers; `docs/features/rfc-status.md` regenerates from it.

RFC text read (`rfc/full/rfc7296.txt`):

| Section | Quote that binds this spec |
|---------|----------------------------|
| 1.3 | "It MAY be initiated by either end of the IKE SA after the initial exchanges are completed." |
| 1.3 | "The responder sends a NO_ADDITIONAL_SAS notification to indicate that a CREATE_CHILD_SA request is unacceptable because the responder is unwilling to accept any more Child SAs on this IKE SA." |
| 1.3.1 | "HDR, SK {SA, Ni, [KEi,] TSi, TSr}" (request) and "HDR, SK {SA, Nr, [KEr,] TSi, TSr}" (response) |
| 1.3.1 | "A failed attempt to create a Child SA SHOULD NOT tear down the IKE SA: there is no reason to lose the work done to set up the IKE SA." |
| 1.3.2 | "HDR, SK {SA, Ni, KEi}" (IKE SA rekey request: no TS payloads), and "Once a peer receives a request to rekey an IKE SA or sends a request to rekey an IKE SA, it SHOULD NOT start any new CREATE_CHILD_SA exchanges on the IKE SA that is being rekeyed." |
| 2.21.2 | "If authentication has succeeded in the IKE_AUTH exchange, the IKE SA is established; however, establishing the Child SA or requesting configuration information may still fail.  This failure does not automatically cause the IKE SA to be deleted.  Specifically, a responder may include all the payloads associated with authentication (IDr, CERT, and AUTH) while sending error notifications for the piggybacked exchanges (FAILED_CP_REQUIRED, NO_PROPOSAL_CHOSEN, and so on), and the initiator MUST NOT fail the authentication because of this.  The initiator MAY, of course, for reasons of policy later delete such an IKE SA." |
| 2.21.3 | "After the IKE SA is authenticated, all requests having errors MUST result in a response notifying the other end of the error." |

**Key insights:**
- Ze holds exactly one Child SA per PeerSession (`reconcile.go` `childSA`, `setChildSA`, `getChildSA`); "create a new Child SA" here means "create THE Child SA when the session holds none".
- The responder today answers a child failure in IKE_AUTH with the notify only and `StateDead` (`responder.go` `handleAuthRequest`, whose comment says "Ze still tears the IKE SA down"); strongSwan then never sees AUTH.
- `handleCreateChildSAOwned` (`inbound.go`) classifies a peer request: REKEY_SA notify means Child rekey; otherwise ANY KE payload means IKE SA rekey; otherwise refuse as a new Child SA. A new Child SA request WITH PFS (KEi plus TSi and TSr) is therefore misread as an IKE SA rekey today. Section 1.3.2's IKE rekey request has no TS payloads; that is the discriminator.
- strongSwan `handle_child_sa_failure` (`src/libcharon/sa/ikev2/tasks/child_create.c`; a fetched copy is `ss_child_create.c` in the parent session scratch) keeps the IKE SA unless `close_ike_on_child_failure` is set.
- libreswan was not compared (subagent budget); strongSwan is the interop peer and its default is the behavior to meet.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/inbound.go` (`handleCreateChildSAOwned`) - the response side dispatches by `pendingRekey.kind` (rekeyChild, rekeyIKE); the request side sends REKEY_SA to `respondChildRekey`, a KE payload to `respondIKERekey`, and anything else to NO_PROPOSAL_CHOSEN, "asks for a NEW Child SA, which Ze does not create".
  → Constraint: the refusal routing on responses (unrecognized error notify fails the request, TEMPORARY_FAILURE hold, NO_ADDITIONAL_SAS re-establish, CHILD_SA_NOT_FOUND re-establish, other refusal `refuseRekey`) is preserved unchanged for the two rekey kinds.
- [ ] `internal/component/ike/engine/rekey.go` (`initiateChildRekey`, `applyChildRekeyResponse`, `childRekeyKeys`, `respondChildRekey`, `newRekeyedChild`, `childRekeyDHGroup`, `buildChildSAPayloads`, `rekeyRefusalRecord`) - every builder derives the new Child SA from an OLD one (addresses, IfID, ReqID, Owner, UDPEncap, ports, Mode, ESPGroup, PolicyPriority).
  → Decision: creation needs a builder with no old child; its field sources are the ones `createFirstChildSA` (`child.go`) already takes from the SA and the peer config (table under Data Flow).
- [ ] `internal/component/ike/engine/child.go` (`initiatorFirstChildSA`, `createFirstChildSA`) - keys from Ni and Nr of IKE_SA_INIT; refuses `ChildOutboundSPI == 0`; the address source honors MOBIKE; Mode from `sa.UseTransportMode`; `UDPEncap` is NATDetected or local port 4500; `ReqID` is `defaultReqID`.
- [ ] `internal/component/ike/engine/established.go` (`runEstablished`, `maintainSA`, `startChildRekey`, `cleanupChild`, `reapStalePending`) - the initiator calls `initiatorFirstChildSA` and ends the cycle on failure; the responder returns `errInvalidMessage` when it holds no child; `emitChildUp` and `emitRouteAdd` dereference the child; `childLT` starts at establishment.
  → Constraint: a nil child must be a legal state through the whole loop: the route re-announce arm (already nil-guarded), Child soft and hard lifetime, `cleanupChild`, and the supersede promotion in `reapStalePending`.
- [ ] `internal/component/ike/engine/fsm.go` (`runInitiator`, `handleAuthResponse`) - `handleAuthResponse` records `SA.ChildRefusal` from the first error notify when there is no SAr2 and no EAP payload; `runInitiator` then calls `deleteChildlessIKESA`.
- [ ] `internal/component/ike/engine/delete.go` (`deleteChildlessIKESA`, `closeDesignatedChildSAs`) - a peer Delete of the live Child SA sets `sessionChildDown`, which ends the owner loop and re-establishes.
- [ ] `internal/component/ike/engine/responder.go` (`handleAuthRequest`, `buildAuthResponse`, `respondAuthError`) - the IKE_AUTH response (IDr, CERT, AUTH, SAr2, TSi, TSr) is built and the Child SA installed in one function; any child error returns before AUTH is built.
- [ ] `internal/component/ike/engine/reconcile.go` (`PeerSession`, `setChildSA`, `Info`, `stopPeerSession`) - `pendingRekey` and `supersededChild` are owner-loop-only; `childRekeyHoldUntil`, `childRekeyRefusedUntil`, `ikeRekeyHoldUntil` and `ikeRekeyRefusedUntil` are per-kind holds.
- [ ] Readers of `getChildSA()` that must tolerate nil: `espInstalled` (`metrics.go`), `closeDesignatedChildSAs` (`delete.go`), `migrateMobikeChild` (`mobike.go`), `handleCreateChildSAOwned` (`inbound.go`), `runEstablished`, `maintainSA`, `startChildRekey`, `reapStalePending`, `cleanupChild` (`established.go`), `Info`, `stopPeerSession` (`reconcile.go`).
- [ ] Tagged tests in `rfc7296_payload_order_test.go` (RFC7296-2.5-13), `rfc7296_auth_test.go` and `rfc7296_remote_cert_chain_test.go` (RFC7296-1.2-2) - fixtures answer IKE_AUTH with no SAr2 and no notify and assert `StateEstablished`.
- [ ] `internal/le/interoplab/ipsec/checkers.go` - checkers already run `swanctl --initiate --child` and `--terminate --ike`, and `checkPeerReloadNarrowing` writes a conf then runs `swanctl --load-conns` mid-scenario.

**Behavior to preserve:**
- Child SA rekey and IKE SA rekey, both roles, including every refused-rekey rule landed in 520b85543c..c880ed7d30 and the journal row of 25b6027287.
- The IKE_AUTH Child SA path when it succeeds (keys from Ni and Nr, no KE).
- Authentication failure still answers AUTHENTICATION_FAILED and kills the SA.
- A peer Delete of the live Child SA still ends the owner loop and re-establishes (unless the owner decides otherwise, Q-5).
- `show vpn ipsec sa` output keys and the `sa-up`, `child-up` and `child-rekey` events.

**Behavior to change:**
- Responder: a child failure after a verified AUTH answers IDr, CERT when X.509, AUTH, plus the error notify, and keeps the IKE SA established and childless.
- Initiator: an authenticated IKE_AUTH response without a Child SA keeps the IKE SA (no Delete, `sa-up` emitted) and schedules CREATE_CHILD_SA creation. `deleteChildlessIKESA` and `errChildSARefused` are deleted (no-layering).
- Request classification: TS payloads present and no REKEY_SA means a new Child SA, with or without KEi.
- A new Child SA request on a session with no Child SA is answered and the Child SA installed; on a session that holds one it is refused with NO_ADDITIONAL_SAS (Q-4).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire: an IKE_AUTH response (initiator) or request (responder) on UDP 500 or 4500; a CREATE_CHILD_SA request or response on an established SA, routed by `dispatchInbound` to the owner loop's `ps.inbound` channel.
- Timer: the owner loop's one-second ticker drives the creation retry schedule on a childless SA.

### Transformation Path
1. IKE_AUTH: AUTH is verified first; the child part is evaluated after. Responder: a child selection failure produces the authenticated response with the notify in place of SAr2, TSi and TSr. Initiator: no SAr2 means childless establishment; the notify, if any, is logged.
2. `runEstablished` enters `maintainSA` with or without a Child SA; `sa-up` is emitted in both cases, `child-up` and the route add only with a child.
3. Owner tick, childless, initiator role: when the creation hold has passed, the request window is free, and no IKE rekey is pending or awaiting its swap, build and send the creation request; record it in `ps.pendingRekey` under a new creation kind.
4. Response to the creation request: refusal routing (AC-6); on success derive keys (Section 2.17, Ni and Nr of THIS exchange, plus the D-H secret when KE was exchanged), build the Child SA from the SA (no old child), install, `setChildSA`, emit `child-up` and the route add, start the Child lifetime.
5. Peer creation request: classify (REKEY_SA, then TS present, then KE-only IKE rekey); a creation on a childless session narrows TS, selects ESP and the D-H group per the esp-group, answers, installs.

New-child builder field sources (no old child):

| Field | Source |
|-------|--------|
| LocalAddr / RemoteAddr | peer config addresses, the MOBIKE current path when enabled (as `createFirstChildSA`) |
| IfID | `resolveIfID` over the peer config |
| ReqID | `defaultReqID` |
| Owner / PolicyPriority | `sa.PeerName` / `sa.PeerCfg.PolicyPriority` |
| UDPEncap / NATDetected / UDP ports | `sa.NATDetected`, local port 4500, MOBIKE ports (as `createFirstChildSA`) |
| Mode | the mode THIS exchange negotiated (USE_TRANSPORT_MODE request and echo) |
| Selectors / TSLocal / TSRemote | the selectors negotiated in THIS exchange, oriented by its initiator (Section 2.9, `ts_narrow.go`), with NAT transport substitution (`ts_nat_substitute.go`) |
| ESPGroup | the accepted proposal of THIS exchange |
| Keys | KEYMAT from SK_d, Ni and Nr of this exchange, plus the D-H shared secret when KE was exchanged |
| LocalIsInitiator | true when Ze initiated THIS CREATE_CHILD_SA exchange, whatever its IKE SA role |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> dataplane | `installChildSA` (XFRM or VPP), unchanged | No |
| Engine -> event bus | `sa-up`, `child-up`, route add and withdraw; event types unchanged | No |
| Wire -> engine | the CREATE_CHILD_SA payload chain through `handleCreateChildSAOwned` | No |

### Integration Points
- `handleCreateChildSAOwned` - gains the creation response branch and the creation request branch.
- `maintainSA` ticker - gains the childless creation trigger beside `startChildRekey`.
- `rekeyRefusalRecord` and the holds - a third per-kind record and hold pair, for creation.
- `buildChildSAPayloads`, `narrowChildSelectors`, the responder ESP selection, `childRekeyKeys`, `installChildSA` - reused, not copied.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | creation runs on the owner loop through the same request window and dispatch as rekey |
| No unintended coupling (components stay isolated) | Yes (design) | engine-only change |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | the no-old-child builder takes over the field derivation inside `createFirstChildSA`, which then calls it; rekey keeps `newRekeyedChild` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | control plane, one exchange per minutes |
| Registration over hardcoding, outbound | N-A | no command, view, family or handler is added |
| Registration over hardcoding, inbound | Yes (design) | the only enumeration touched is the CREATE_CHILD_SA request classifier, whose cases are the protocol's own discriminators (REKEY_SA, TS, KE), not a feature list; `scenarioCheckers` and the parity list gain entries as every scenario does |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | strongSwan as initiator, given a responder that answers IDr, AUTH and NO_PROPOSAL_CHOSEN, keeps the IKE SA and initiates later children with CREATE_CHILD_SA | `child_create.c` `handle_child_sa_failure`, `close_ike_on_child_failure` default FALSE | the responder interop scenario cannot reach the creation path | the `childless-ike-sa-ze-responder` scenario sees charon log "failed to establish CHILD_SA, keeping IKE_SA" | unvalidated |
| A-2 | strongSwan as responder accepts a CREATE_CHILD_SA new-child request on an IKE SA whose IKE_AUTH child it refused | RFC 7296 Section 1.3; strongSwan child_create responder path | the initiator scenario fails at creation | the `childless-ike-sa-ze-initiator` scenario | unvalidated |
| A-3 | The interop checker can rewrite strongSwan's conn mid-scenario and reload it | `checkPeerReloadNarrowing` writes `zz-narrow.conf` then runs `swanctl --load-conns` | the initiator scenario needs another way to turn the refusal into acceptance | read of `checkPeerReloadNarrowing` (done) | validated |
| A-4 | A `.ci` can reload the responder ze's esp-group mid-test | `test/ipsec/ipsec-peer-reload-applies-selectors.ci` reloads a peer | the functional test can show only the childless hold, not the recovery | read that `.ci` at implementation start | unvalidated |
| A-5 | No caller outside the engine dereferences the Child SA of an established session without a nil check | the `getChildSA()` reader list above | a nil dereference on a childless session | a childless unit case per reader, plus `show vpn ipsec sa` on a childless session in the `.ci` | unvalidated |
| A-6 | The tagged fixtures RFC7296-2.5-13 and RFC7296-1.2-2 assert only `StateEstablished` and the AUTH outcome, not the Child SA | read of the three test files | treating no-SAr2 as childless would still redden them, forcing an owner-approved edit | run the three tests unedited after phase 2 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A nil Child SA reaches a reader that dereferences it (metrics, MOBIKE migration, supersede promotion, cleanup) | panic in a childless unit test or the `.ci` | phase 2 audits every reader in the list and adds a childless case per reader |
| R-2 | Simultaneous creation: both ends create a Child SA at once (a strongSwan responder may initiate on an acquire) and Ze ends with two | two `child-up` for one peer; two SPI pairs in `show` | when our creation response arrives and the session already holds a Child SA, the new pair is not installed and is deleted with a Delete (AC-12) |
| R-3 | The creation retry loops against a peer that will never accept, filling logs and spending the request window | repeated "create refused" log lines | capped backoff (Q-2); every attempt reserves the single request window, so DPD and rekey interleave |
| R-4 | A KE-bearing new-child request is still read as an IKE SA rekey by a second classifier | a PFS creation answered with IKE SA keys | grep every `hasKEPayload` and `hasRekeySANotify` reader; AC-8 unit test |
| R-5 | Creation started while an IKE SA rekey is pending or its swap is held, against Section 1.3.2's SHOULD NOT | a creation request sent with `pendingIKESwap` set | trigger guard (AC-11) |
| R-6 | Responder IKE_AUTH split: today the child is installed inside `buildAuthResponse` before AUTH is computed, so separating "child failed" from "AUTH built" can reorder the install and the response | a child installed for a response never sent, or AUTH sent beside a stale child | compute IDr and AUTH first, then attempt the child, then assemble either SAr2, TSi and TSr or the notify; the unit test asserts payload order and that no Child SA is installed on refusal |
| R-7 | The EAP responder path (`fromEAP`) takes the same child step and is left on the old behavior | an EAP peer still dies on a child refusal | AC-1 covers the PSK, X.509 and final EAP IKE_AUTH |
| R-8 | A Child lifetime started at IKE establishment fires a soft expiry right after a late creation | a rekey right after creation | the Child lifetime starts when the Child SA is installed (AC-4) |
| R-9 | A Ze pair where the responder refused by policy stays childless, because only the initiator retries | childless forever | intended: the operator sees the state in `show vpn ipsec sa` and the log; a config reload makes the next attempt succeed (functional test) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | IPsec tunnels against strongSwan or Ze peers: a childless IKE SA that never gets a Child SA carries no traffic while it looks up at the IKE layer |
| How is it reverted? | single commit revert; no config change |
| Who else touches this path? | `plan/pre-release/spec-rfcgate-1b-rfc7296-pilot-deferred-child-rekey-refusal-proof.md` (blocked) on the refusal code in `inbound.go` and `rekey.go`; the refused-rekey commits landed today |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH request whose child is refused (wire, responder) | → | `handleAuthRequest` childless establish | `TestResponderKeepsIKESAWhenChildRefused` |
| IKE_AUTH response with a notify in place of SAr2 (wire, initiator) | → | `handleAuthResponse` and `runEstablished` childless | `TestInitiatorKeepsChildlessIKESA` |
| owner tick on a childless initiator SA | → | creation trigger and request builder | `TestChildlessInitiatorCreatesChildSA` |
| CREATE_CHILD_SA new-child request (wire, responder) | → | `handleCreateChildSAOwned` creation branch | `TestResponderCreatesChildOnChildlessSA` |
| config: two ze instances, responder esp-group mismatched then reloaded | → | full path | `test/ipsec/ipsec-childless-ike-sa-recovers.ci` |
| strongSwan peer | → | full path, both roles | `childless-ike-sa-ze-initiator`, `childless-ike-sa-ze-responder` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze responder; an IKE_AUTH request whose AUTH verifies (PSK, X.509, and the final EAP IKE_AUTH) but whose Child SA cannot be built (no acceptable ESP proposal, TS_UNACCEPTABLE, missing SAi2) | The response carries IDr, CERT when X.509, AUTH, and the error notify, and no SAr2, TSi or TSr; the IKE SA is ESTABLISHED; no Child SA is installed; `sa-up` is emitted and `child-up` is not; `show vpn ipsec sa` lists the peer's IKE SA with no Child SA |
| AC-2 | Ze initiator; an IKE_AUTH response whose AUTH verifies, carrying an error notify in place of SAr2 | The IKE SA is established; no Delete is sent; `sa-up` is emitted; the log names the notify; the session stays in `maintainSA`; `deleteChildlessIKESA` and `errChildSARefused` no longer exist |
| AC-3 | Ze initiator; an authenticated IKE_AUTH response with neither SAr2 nor an error notify | Same as AC-2, logged as a response that carried no Child SA; no "no peer ESP SPI recorded" failure; the tagged tests for RFC7296-2.5-13 and RFC7296-1.2-2 pass unedited (Q-3) |
| AC-4 | Ze initiator holding a childless IKE SA, creation hold elapsed | Sends CREATE_CHILD_SA {SA, Ni, [KEi], TSi, TSr}, plus USE_TRANSPORT_MODE under `mode transport`; on a valid response installs the Child SA keyed per Section 2.17 from this exchange's nonces (and D-H secret), emits `child-up` and the route add, and starts the Child lifetime at install; the Child SA then rekeys like any other |
| AC-5 | Creation with `pfs` enabled in the esp-group | KEi is present in the configured group and that group is offered in the SA payload (RFC7296-1.3-1); a success response without KEr is refused; INVALID_KE_PAYLOAD naming another configured group retries in that group; with `pfs` disabled no KEi is sent |
| AC-6 | A creation response is an error | TEMPORARY_FAILURE: hold 60 s; NO_ADDITIONAL_SAS: re-establish the IKE SA; INVALID_KE_PAYLOAD per AC-5; NO_PROPOSAL_CHOSEN, TS_UNACCEPTABLE, or any other or unrecognized error: the IKE SA is kept and the next attempt follows the retry schedule (Q-2). A refused creation never sets the Child or IKE rekey holds |
| AC-7 | Ze, in either IKE role, holding a childless IKE SA receives a CREATE_CHILD_SA new-child request, with or without KEi | Narrows TS (RFC7296-2.9-1 and -2), selects one ESP proposal, answers SA, Nr, [KEr], TSi, TSr, and USE_TRANSPORT_MODE when that was asked and accepted; installs the Child SA; emits `child-up`; a refusal answers the matching error notify (RFC7296-2.21.3-1) and keeps the IKE SA |
| AC-8 | A peer CREATE_CHILD_SA request carrying KEi, TSi and TSr and no REKEY_SA | Handled as a new Child SA, never as an IKE SA rekey; a request with SA, Ni and KEi and no TS stays an IKE SA rekey |
| AC-9 | A peer new-child request while Ze holds a Child SA | Answered with NO_ADDITIONAL_SAS (Q-4); the IKE SA and the live Child SA are unchanged |
| AC-10 | A childless IKE SA over time | DPD probes and answers; an IKE SA rekey succeeds and the replacement stays childless with creation still scheduled; the IKE hard lifetime ends it; a peer IKE Delete and an operator `clear` end it with no Child cleanup error; a MOBIKE update with no Child SA succeeds; metrics and `show` report the peer with no Child SA |
| AC-11 | Creation due while an IKE SA rekey is pending, a peer IKE SA rekey awaits its swap, or the request window is held | No creation request is sent until those clear (Section 1.3.2) |
| AC-12 | Our creation response (success) arrives while the session already holds a Child SA, created by the peer meanwhile | The new pair is not installed; Ze sends a Delete for it; the existing Child SA is untouched |
| AC-13 | Interop: strongSwan initiator, its first child refused by Ze in IKE_AUTH, its second child initiated after | strongSwan keeps the IKE SA and creates the second child through CREATE_CHILD_SA with PFS; traffic passes through it |
| AC-14 | Interop: Ze initiator, strongSwan responder refusing the IKE_AUTH child, then reloaded to accept | Ze keeps the IKE SA (strongSwan lists it ESTABLISHED, no Delete in its log), and Ze's scheduled creation, with KEi, installs the Child SA; traffic passes |
| AC-15 | The peer deletes the live Child SA | Unchanged: the owner loop ends and re-establishes (Q-5) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator peers Ze (responder) with a strongSwan site that brings up a second child on demand | wire IKE_AUTH -> childless establish -> CREATE_CHILD_SA request -> install | `childless-ike-sa-ze-responder` |
| 2 | Operator's Ze initiator meets a peer whose child policy is wrong at first and fixed later | wire IKE_AUTH -> childless -> owner tick creation -> install | `childless-ike-sa-ze-initiator`, `ipsec-childless-ike-sa-recovers.ci` |
| 3 | Operator reads the state of a childless tunnel | `show vpn ipsec sa` | `ipsec-childless-ike-sa-recovers.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResponderKeepsIKESAWhenChildRefused` | `internal/component/ike/engine/childless_test.go` | AC-1 payload set and order, ESTABLISHED, nothing installed; tagged RFC7296-2.21.2 (responder half) | |
| `TestResponderEAPKeepsIKESAWhenChildRefused` | same | AC-1 on the final EAP IKE_AUTH | |
| `TestInitiatorKeepsChildlessIKESA` | same | AC-2, no Delete sent; tagged RFC7296-2.21.2-2 positive, and negative (a corrupted AUTH beside the notify still fails) | |
| `TestInitiatorNoSAr2NoNotifyIsChildless` | same | AC-3 | |
| `TestChildlessInitiatorCreatesChildSA` | `internal/component/ike/engine/create_child_test.go` | AC-4 request payloads, key order, install, lifetime start | |
| `TestCreateChildCarriesKEiWhenPFS` | same | AC-5; RFC7296-1.3-1 | |
| `TestCreateChildInvalidKERetriesNamedGroup` | same | AC-5 | |
| `TestCreateChildRefusalRouting` | same | AC-6 rows; per-kind holds independent | |
| `TestResponderCreatesChildOnChildlessSA` | same | AC-7 with and without KEi, transport mode echo | |
| `TestNewChildWithKEIsNotAnIKERekey` | same | AC-8 | |
| `TestNewChildWhileChildHeldAnswersNoAdditionalSAs` | same | AC-9 | |
| `TestChildlessSAMaintenance` | same | AC-10 readers: metrics, MOBIKE, cleanup; an IKE rekey keeps creation scheduled | |
| `TestCreateChildWaitsForIKERekey` | same | AC-11 | |
| `TestCreateChildCollisionDeletesNewPair` | same | AC-12 | |
| `TestCreateChildRetrySchedule` | same | Q-2 schedule, reset on success | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Creation backoff | initial to cap (Q-2: 30 s to 300 s) | 300 s | N-A | never exceeds the cap |
| Peer ESP SPI in a creation answer or request | 1 to 2^32-1 | 1 | 0 refused (RFC 4303 Section 2.1) | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-childless-ike-sa-recovers` | `test/ipsec/ipsec-childless-ike-sa-recovers.ci` | two ze instances; the responder's esp-group shares nothing with the initiator's; `sa-up` fires, `show vpn ipsec sa` shows the peer with no Child SA and no reconnect; the responder's esp-group is reloaded to match; `child-up` fires on the initiator from a CREATE_CHILD_SA creation | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `childless-ike-sa-ze-responder` | `test/interop-ipsec/scenarios/childless-ike-sa-ze-responder/` | strongSwan initiator with two children, `start_action = none`: `refused` (an ESP proposal Ze refuses), then `wanted` (with a PFS group); the checker initiates `refused`, then `wanted` | AC-1, AC-7, AC-8, AC-13: Ze authenticates while refusing the child, keeps the IKE SA, and creates the second child from a KE-bearing request; ping through `wanted` | |
| `childless-ike-sa-ze-initiator` | `test/interop-ipsec/scenarios/childless-ike-sa-ze-initiator/` | strongSwan responder whose child ESP proposal Ze does not offer; the checker asserts the IKE SA stays ESTABLISHED with no CHILD_SA past one retry, then rewrites the conn to match and runs `swanctl --load-conns` | AC-2, AC-4, AC-5, AC-14: no Delete, then Ze's creation (PFS on) installs the child; ping | |

Both run only on a Docker host that passes the Ze kernel check (the QEMU docker-lab route). Each is proven discriminating per `ai/rules/interop-and-goal-validation.md`: revert the change, rebuild the ze image, observe red, restore, observe green, record the red.

## Files to Modify
- `internal/component/ike/engine/responder.go` - `handleAuthRequest`, `buildAuthResponse`: AUTH first, child second, notify response with IDr, CERT and AUTH; childless finish
- `internal/component/ike/engine/fsm.go` - `handleAuthResponse` childless establish (AC-2, AC-3); `runInitiator` drops the childless delete; `errChildSARefused` removed
- `internal/component/ike/engine/delete.go` - `deleteChildlessIKESA` removed
- `internal/component/ike/engine/sa.go` - `ChildRefusal` kept only if the log still reads it, otherwise removed
- `internal/component/ike/engine/established.go` - `runEstablished` childless in both roles; `maintainSA` creation trigger; Child lifetime starts at install; nil-safe readers
- `internal/component/ike/engine/inbound.go` - request classifier (REKEY_SA, TS, KE), creation request branch, creation response branch, NO_ADDITIONAL_SAS while a child is held
- `internal/component/ike/engine/rekey.go` - creation request builder and response applier reusing `buildChildSAPayloads`, `childRekeyKeys` and `rekeyRefusal`; the new pending kind; its per-kind refusal record
- `internal/component/ike/engine/child.go` - the no-old-child builder; `createFirstChildSA` derives its fields through it
- `internal/component/ike/engine/reconcile.go` - creation holds and retry state; nil-safe `Info` and `stopPeerSession`
- `internal/component/ike/engine/metrics.go`, `internal/component/ike/engine/mobike.go` - nil Child SA
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the Section 2.21.2 paragraph and its anchors
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - a new section on creating a Child SA on an established IKE SA, and creation rows in the refusal table
- `docs/architecture/ike/ipsec-14-responder.md` - responder childless IKE_AUTH
- `docs/architecture/ike/ipsec-10-cli-diag.md` - the childless `PeerInfo` and `show vpn ipsec sa` row
- `rfc/short/rfc7296.md` - Support coverage sentence for Sections 1.3.1 and 2.21.2
- `features/ikev2-engine.md` - scope sentence
- `internal/le/interoplab/ipsec/checkers.go`, `test/interop-ipsec/parity_test.go` - two scenario checkers
- `plan/journal/silent-fall-through.md` - the 2026-10-10 row marked FIXED with its producer

## Files to Create
- `internal/component/ike/engine/childless_test.go`, `internal/component/ike/engine/create_child_test.go`
- `test/ipsec/ipsec-childless-ike-sa-recovers.ci`
- `test/interop-ipsec/scenarios/childless-ike-sa-ze-responder/` (swanctl.conf, ze.conf)
- `test/interop-ipsec/scenarios/childless-ike-sa-ze-initiator/` (swanctl.conf, ze.conf)
- `rfc/discrimination/rfc7296.json` entries for the new tags, written by `./le rfc discriminate-record`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf (Q-1 recommends no policy knob) |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | `show vpn ipsec sa` already lists SAs; only its childless row content changes |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | N-A | no RPC; the behavior `.ci` is listed above |
| Pipe completeness | N-A | no new output command |
| Env var registration | No | none added |
| Doctor check for runtime dependencies | No | no new path, port, module or binary |
| Prometheus counters/metrics | No | the existing per-peer gauges read a nil Child SA as no child; the creation outcome is logged |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/ikev2-engine.md` |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | checked at implementation: grep `docs/guide/` for "Child SA" and correct any page that says a child failure tears the tunnel down |
| 7 | Wire format changed? | No | the CREATE_CHILD_SA payloads already exist |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7296.md` Support coverage; `docs/features/rfc-status.md` regenerated |
| 10 | Test infrastructure changed? | No | two scenarios follow the existing pattern |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-7-ikev2-engine.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-14-responder.md`, `docs/architecture/ike/ipsec-10-cli-diag.md` (what `PeerInfo` and `show vpn ipsec sa` report for a childless peer) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | events unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation by `./le spec citation anchors spec plan/immediate/spec-ike-create-child-sa-on-established-ike-sa.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

Discovery (`ai/rules/repo-maintenance.md`): an agent finds this work from the `ai/INDEX.md` IKE and IPsec rows, which point at `docs/architecture/ike/`; regression is held by the tagged tests and by the two interop scenarios in `scenarioCheckers` and the parity list; the RFC ledger row RFC7296-2.21.2-2 carries the proof.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- failing tests for each Wiring Test row; the responder and initiator childless paths reachable from IKE_AUTH.
   - Tests: the four wiring unit tests
   - Files: `responder.go`, `fsm.go`, `established.go`
   - Verify: tests red for the stated reason
2. **Phase: Childless IKE SA** -- AC-1, AC-2, AC-3, AC-10, AC-15; delete `deleteChildlessIKESA` and `errChildSARefused`; nil-safe readers; Child lifetime at install; `ipsec-7` and `ipsec-14` pages in this phase.
3. **Phase: Responder creation** -- AC-7, AC-8, AC-9; the classifier; the no-old-child builder; the `ipsec-8` page in this phase.
4. **Phase: Initiator creation** -- AC-4, AC-5, AC-6, AC-11, AC-12; the retry schedule; refusal routing.
5. **Phase: Functional and interop** -- the `.ci`, the two scenarios, the discrimination records, the `rfc/short` Support cell, `features/ikev2-engine.md`, the journal row update.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named function |
| Correctness | KEYMAT order for a created Child SA follows the CREATE_CHILD_SA exchange role, not the IKE SA role |
| Correctness | the classifier never sends a TS-bearing request to `respondIKERekey` |
| Correctness | the refused-rekey behavior of both rekey kinds is unchanged (existing tests untouched and green) |
| Rule: no-layering | `deleteChildlessIKESA`, `errChildSARefused` and the responder branch that "still tears the IKE SA down" are deleted, not bypassed |
| Rule: rfc-compliance | every new implementing function quotes its RFC sentence; every caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| childless and creation unit tests | the scoped `./le job run` package test of `internal/component/ike/engine` |
| functional test | `./le test functional` selecting `ipsec-childless-ike-sa-recovers` |
| interop scenarios | `./le test integration interop-ipsec` with the two scenario selectors, on a Docker host that passes the kernel check |
| old path deleted | a grep for `deleteChildlessIKESA` and `errChildSARefused` returns nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the child decision is taken only after AUTH verifies; an unauthenticated IKE_AUTH message can neither keep nor create anything |
| Resource exhaustion | one Child SA per session (NO_ADDITIONAL_SAS beyond it); capped retry; creation holds the single request window |
| Key hygiene | a refused or collided creation clears its D-H private value and derived keys (Section 2.12) |

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

## Owner Questions (design gate)

| ID | Question | Recommendation |
|----|----------|----------------|
| Q-1 | Replace the f0006faaa2 fallback (delete the childless IKE SA) or keep it as a policy option? | Replace: delete `deleteChildlessIKESA` and `errChildSARefused` in phase 2 (no-layering); no config knob |
| Q-2 | Initiator creation retry schedule on a childless SA | first attempt 30 s after establishment, doubling, capped at 300 s, reset on success; TEMPORARY_FAILURE holds 60 s; NO_ADDITIONAL_SAS re-establishes |
| Q-3 | Journal row (a response with neither SAr2 nor a notify): treat it as childless, so the RFC7296-2.5-13 and RFC7296-1.2-2 tagged tests stay as they are, or edit those fixtures to carry SAr2? | Treat it as childless; no tagged-test edit needed |
| Q-4 | The peer asks for a new Child SA while Ze holds one | Answer NO_ADDITIONAL_SAS (Section 1.3's meaning) instead of today's NO_PROPOSAL_CHOSEN; several Child SAs per IKE SA is a separate spec |
| Q-5 | The peer deletes the live Child SA: keep today's teardown and re-establish, or keep the IKE SA childless and recreate? | Keep today's behavior in this spec |
| Q-6 | Does a Ze RESPONDER also initiate creation on a childless SA? | No: only the IKE SA initiator retries, which avoids creation collisions between two Ze peers; the responder serves the peer's requests |
| Q-7 | Bucket | `plan/immediate/`: an operator facing strongSwan meets it as a tunnel that never comes up |

→ Decision (owner, 2026-10-10): the instruction to implement approves Q-1 to Q-7 as recommended above. Q-1: `deleteChildlessIKESA` and `errChildSARefused` are deleted, no knob. Q-2: 30 s, doubling, capped at 300 s, reset on success; TEMPORARY_FAILURE waits 60 s; NO_ADDITIONAL_SAS re-establishes. Q-3: no SAr2 and no notify is childless; the RFC7296-2.5-13 and RFC7296-1.2-2 tests stay unedited. Q-4: NO_ADDITIONAL_SAS. Q-5: today's teardown and re-establish. Q-6: only the IKE initiator retries creation. Q-7: `plan/immediate/`.

## Design Insights

- The KE-based classifier in `handleCreateChildSAOwned` misreads a PFS new-child request as an IKE SA rekey. Today it only bites a request Ze refuses anyway; this spec makes it reachable, so it is fixed here as AC-8.
- A peer SPI of 0 in IKE_AUTH SAi2: RFC 7296 leaves the notify open. Section 3.10.1 lets NO_PROPOSAL_CHOSEN answer "any case where the offered proposals (including but not limited to SA payload values, USE_TRANSPORT_MODE notify, IPCOMP_SUPPORTED notify) are not acceptable for the responder", and INVALID_SYNTAX answer a message where "some type, length, or value was out of range". Section 2.21.2 names INVALID_SYNTAX among "the only ones to cause the IKE SA to be deleted or not created", so INVALID_SYNTAX would undo the childless IKE SA. Ze keeps NO_PROPOSAL_CHOSEN; nothing changed. The CREATE_CHILD_SA rekey path answers SPI 0 with INVALID_SYNTAX (`errMalformedRequest`), which Section 3.10.1 also permits.
→ Decision (owner, 2026-10-10, option A): `TestResponderRefusesPeerSPIZero` asserts no Child SA and a NO_PROPOSAL_CHOSEN refusal with no SA payload; the clause that the IKE SA must not establish is dropped, because RFC 4303 Section 2.1 and RFC 3948 Section 2.1 constrain the ESP SPI, not the IKE SA.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One Child SA per session; creation only when none is held | full multi-child support | the session, the shared dataplane policies, metrics and events are built around one Child SA; multi-child is a separate, larger feature |
| Only the IKE SA initiator retries creation | both roles retry | a Ze pair would collide on every recovery; a strongSwan initiator drives its own creation |
| Reuse the rekey request window, pending slot and refusal routing with a third kind | a separate creation state machine | RFC 7296 Section 2.3 gives one request window; the refusal table is already the contract |
| Classify by REKEY_SA, then TS presence, then KE | KE presence first (today) | Section 1.3.2's IKE rekey request carries no TS payloads; Section 1.3.1's creation request always does |
| Keep the childless IKE SA (Section 2.21.2 and 1.3.1's SHOULD NOT) | keep f0006faaa2's delete-and-back-off | strongSwan's default keeps it; deleting throws away a working IKE SA and makes a peer that creates children on demand unreachable |

## Known Limitations
- More than one Child SA per IKE SA: not done; Ze answers NO_ADDITIONAL_SAS. It needs its own spec if the owner wants it.
- RFC 6023 childless initiation (no SAi2 in IKE_AUTH, CHILDLESS_IKEV2_SUPPORTED): not done; it needs its own spec.

## RFC Documentation (Scope: protocol)

Add `// RFC 7296 Section X.Y: "<quoted requirement>"` above enforcing code, using
the quotes in the Required Reading table: Section 1.3.1 on the creation builder,
the responder and the applier; Section 1.3.1's SHOULD NOT on the branches that keep
the IKE SA; Section 2.21.2 on both IKE_AUTH paths; Section 1.3.2 on the classifier
and the creation trigger guard; Section 1.3 on NO_ADDITIONAL_SAS. Callers name the
section.

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
- [ ] AC-1..AC-15 all demonstrated
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script

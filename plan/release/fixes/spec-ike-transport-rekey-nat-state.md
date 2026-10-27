# Spec: ike-transport-rekey-nat-state

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped IKEv2 engine loses RFC 7296 Section 2.23.1 NAT handling for a transport-mode Child SA rekeyed after its IKE SA was rekeyed, on both roles, and installs a mode the rekey exchange did not negotiate |
| RFC gaps | - |
| Depends | - |
| Phase | 1/4 |
| Approved | 2026-10-27, supervising session f3ae7c00 under the owner's standing rule for away goals (`ai/META-DECISIONS.md`, "How agents work"): every design question is answered by the RFC 7296 text quoted here (Sections 1.3.1, 1.3.3, 2.8, 2.23.1). No question is left for Thomas |
| Handoff | - |
| Updated | 2026-10-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The mode of a Child SA is held in two places: the IKE SA's flags
`SA.UseTransportMode` and `SA.PeerRequestedTransport`, and the Child SA's own
`ChildSA.Mode`. The flags are the negotiation outcome of a CREATION exchange
(IKE_AUTH, or a CREATE_CHILD_SA that creates a Child SA), but later exchanges
read them as if they described the installed Child SA. An IKE SA rekey builds
the successor SA without them (RFC 7296 Section 2.8: "An IKE SA so created
inherits all of the original IKE SA's Child SAs"), so after it:

| Reader | What goes wrong after an IKE SA rekey |
|--------|---------------------------------------|
| `respondChildRekey` -> `narrowChildSelectorsIn` -> `substituteResponderSelectors`, `keepSingleAddress`, `oneTransportPair` (gated on `transportSelectorsInPlay`) | A NATed peer's transport-mode Child SA rekey gets no Section 2.23.1 substitution, so the SPD lookup (policy match) intersects nothing and Ze answers TS_UNACCEPTABLE; a wide proposal is not held to one address |
| `applyChildRekeyResponse` -> `recordInitiatorSelectors` -> `substituteInitiatorSelectors` and the one-pair refusal (`sa.UseTransportMode && len(pairs) > 1`) | Ze's own transport-mode Child SA rekey reads the answer unsubstituted and refuses it as widened, or installs a multi-pair transport answer it must refuse |
| `acceptMobikeUpdate` (`mobike.go`, `sa.UseTransportMode`) | An UPDATE_SA_ADDRESSES for a transport-mode Child SA is accepted, where RFC 4555 leaves transport mode out of scope and Ze refuses it before the IKE SA rekey |

A second defect sits on the same path (journal
`derived-flags-overwrite-what-the-wire-carried.md`, 2026-10-27): the rekey
responder installs the retired Child SA's mode whatever the request carried, so
a rekey request without USE_TRANSPORT_MODE on a transport SA is answered without
the notify (the peer then installs tunnel) while Ze installs transport.

Goal: every exchange judges its traffic selectors in the mode the Child SA it
creates will install, a value computed once per exchange and passed explicitly;
the persistent mode of an installed Child SA lives only in `ChildSA.Mode`; the
IKE SA flags stop being read outside the IKE_AUTH exchange that wrote them, and
`PeerRequestedTransport` is deleted. Proven by tests that run a real IKE SA
rekey, then a real CREATE_CHILD_SA rekey of a NATed transport-mode Child SA in
each role, and assert the substituted selectors, the single-address refusal and
the installed mode.

Found during `spec-ike-unbound-peer-inner-address` (journal row in
`plan/journal/one-state-held-in-two-fields.md`, 2026-10-26).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` (IKE door)
  → Constraint: IKE is a component; the change stays inside `internal/component/ike/engine` and its interop lab (`internal/le/interoplab/ipsec`). No new registry, no cross-component import.
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - SA fields, NAT verdict, MOBIKE owner state
  → Constraint: `NATDetected`, `BehindNAT`, `PeerBehindNAT` are carried across an IKE SA rekey by `inheritSendPath` (`sa.go`); the NAT verdict is NOT the missing state, only the mode is.
  → Decision: the MOBIKE paragraph gains the rule that the transport-mode refusal reads the installed Child SA's mode.
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - Child SA rekey mode paragraph ("A rekey keeps the mode of the Child SA it replaces")
  → Decision: that paragraph is rewritten: the replacement installs the mode the rekey exchange negotiated, transport only when the request carries USE_TRANSPORT_MODE and the retired SA runs transport.
  → Constraint: a transport-mode Child SA carries exactly one pair (one XFRM state selector); the one-pair checks stay, only their gate changes.
- [ ] `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - transport-mode NAT substitution
  → Constraint: "The substitution runs inside `narrowChildSelectors` and `recordInitiatorSelectors`, which are the single entry points of their roles." It stays there; the gate becomes an argument, not a new call site.
  → Decision: "The gate is transport mode AND a NAT verdict" is kept, with "transport mode" defined as the mode the exchange's Child SA installs.

- [ ] `docs/architecture/ike/ipsec-14-responder.md` - responder substitution (line ~205) and the pool-bound decline (lines ~269-274, anchored to `transportSelectorsInPlay`)
  → Decision: the pool-bound paragraph and its `source:` anchor are rewritten in Phase 2: the decline is decided by `decideResponderTransportMode`, and the narrowing gate is the decided mode it returns, so no separate "not in play" predicate exists.
  → Constraint: the responder substitutes before the policy match; that ordering is kept.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - rows RFC7296-2.23.1-1 (exactly one IP address in TSi and TSr for transport mode), -2, -3 (client proposal), RFC7296-1.3.1-1, -1.3.1-2
  → Constraint: the new single-address assertion on the rekey path is tagged RFC7296-2.23.1-1 and owes a discrimination record (`./le rfc record`). The substitution procedure itself is lower-case "should" text and has no row; its tests are untagged.
- [ ] `rfc/full/rfc7296.txt` Sections 1.3.1, 1.3.3, 2.8, 2.23.1 (quoted below)
  → Constraint: Section 2.23.1, client rules: "If transport mode for the SA was selected (that is, if the server included USE_TRANSPORT_MODE notification in its response):" then substitute, and "Do address substitution before using those Traffic Selectors for anything other than storing original content of them." The initiator gate is the mode the answer selected.
  → Constraint: Section 2.23.1, responder: "In this case, the server should first check that the initiator requested transport mode, and then do address substitution on the Traffic Selectors." and "If an entry is found but it does not allow transport mode, then the server MAY undo the address substitution and redo the SPD lookup using the original Traffic Selectors.  If the second lookup succeeds, the server will create an SA in tunnel mode using real Traffic Selectors sent by the other end." Ze decides acceptance from configuration BEFORE the SPD lookup (`decideResponderTransportMode`), so the undo outcome is known in advance: a declined request narrows the real selectors.
  → Constraint: Section 2.23.1: "For transport mode, it MUST use exactly one IP address in the TSi and TSr payloads."
  → Constraint: Section 1.3.1: "Except when using this option to negotiate transport mode, all Child SAs will use tunnel mode." and "If the request is accepted, the response MUST also include a notification of type USE_TRANSPORT_MODE.  If the responder declines the request, the Child SA will be established in tunnel mode."
  → Constraint: Section 1.3.3: "The notifications described in Section 1.3.1 may also be sent in a rekeying exchange.  Usually, these will be the same notifications that were used in the original exchange; for example, when rekeying a transport mode SA, the USE_TRANSPORT_MODE notification will be used." A rekey is a Child SA creation for mode purposes; its request's notify decides its mode.
  → Constraint: Section 2.8: "An IKE SA so created inherits all of the original IKE SA's Child SAs, and the new IKE SA is used for all control messages needed to maintain those Child SAs." The successor SA maintains the inherited Child SAs, so their mode must be read from them.

**Key insights:** (minimal context to resume after compaction)
- The NAT verdict survives an IKE SA rekey (`inheritSendPath`); only the two mode flags are lost (both successor constructors in `rekey.go`, `applyIKERekeyResponse` and `respondIKERekey`, build `&SA{...}` without them).
- `narrowChildSelectorsIn` already receives `mode` (only `dropForeignInnerOverlaps` uses it); the substitution and the single-address filter ignore it and call `transportSelectorsInPlay(sa)`.
- `recordInitiatorSelectors` has no mode argument at all.
- `UseTransportMode` implies `wantsTransportMode(sa)` at every writer, so `childTransportMode` (`wantsTransportMode || UseTransportMode`) already equals `wantsTransportMode`.
- A session holds one live Child SA (`PeerSession.childSA`, `getChildSA`, `reconcile.go`).
- The engine and crypto test packages compile on HEAD as of 2026-10-27 (`go vet` exit 0 under `./le job run`): the ENCR_3DES / ENCR_CHACHA20_POLY1305 hazard was closed by commit ba03d7c2c7, and `internal/component/ike/` had no uncommitted hunks.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/engine/ts_nat_substitute.go` - `transportSelectorsInPlay(sa)` answers `sa.UseTransportMode`, else false for a pool-bound peer, else `sa.PeerRequestedTransport`. `substituteResponderSelectors` and `substituteInitiatorSelectors` return the lists unchanged when it is false, else store originals (`storeOriginalSelectorAddresses`, into `sa.OriginalTSiAddr/OriginalTSrAddr`) and pin by NAT side.
- [ ] `internal/component/ike/engine/ts_narrow.go` - `narrowChildSelectors(sa, tsi, tsr, floor)` calls `narrowChildSelectorsIn(..., decidedChildMode(sa))`; `decidedChildMode` reads `sa.UseTransportMode`. `narrowChildSelectorsIn` substitutes, applies the virtual-IP lease, narrows against `policyPairs(sa.PeerCfg, false)`, then under `transportSelectorsInPlay(sa)` runs `keepSingleAddress` (TS_UNACCEPTABLE when empty) and `oneTransportPair`, then `dropForeignInnerOverlaps(sa, narrowed, mode)`. `recordInitiatorSelectors(sa, tsi, tsr, floor)` substitutes via `substituteInitiatorSelectors`, checks leases, the ceiling (`sa.ProposedChildPairs` or policy), refuses `sa.UseTransportMode && len(pairs) > 1` with `errTSTransportPairs`, then the floor.
- [ ] `internal/component/ike/engine/rekey.go` - `respondChildRekey` parses `transportRequested` from the request, narrows with `narrowChildSelectorsIn(sa, reqTSi, reqTSr, floor, old.Mode)`, builds the replacement with `newRekeyedChild` (copies `Mode: old.Mode`), and echoes USE_TRANSPORT_MODE when `transportRequested && child.Mode == modeTransport`. It sets no SA flag. `applyChildRekeyResponse` refuses a tunnel answer to a transport rekey (`errChildRekeyTransportDeclined`), then calls `recordInitiatorSelectors(sa, respTSi, respTSr, floor)`. `initiateChildRekey` sends USE_TRANSPORT_MODE when `oldChild.Mode == modeTransport` and records `sa.ProposedChildPairs`. `applyIKERekeyResponse` and `respondIKERekey` build the successor SA with neither flag and call `inheritSendPath`.
- [ ] `internal/component/ike/engine/transport_mode.go` - `decideResponderTransportMode(sa)` WRITES `sa.UseTransportMode` (false for a pool-bound peer, else `PeerRequestedTransport && wantsTransportMode`). `recordInitiatorTransportMode(sa, notify)` WRITES `sa.UseTransportMode` and returns whether to delete. `adoptAuthResponseNegotiation` calls it then `recordInitiatorSelectors`.
- [ ] `internal/component/ike/engine/responder.go` - `handleAuthRequest` sets `sa.PeerRequestedTransport` on the notify and calls `decideResponderTransportMode` in the same function; `selectAuthChildSA` narrows through `narrowChildSelectors` and echoes on `sa.UseTransportMode`; `buildAuthResponse` creates the Child SA through `createFirstChildSA`. `responder_eap.go` narrows through `narrowChildSelectors` on the final IKE_AUTH.
- [ ] `internal/component/ike/engine/create_child.go` - `respondNewChild` writes `sa.PeerRequestedTransport = req.transport`, calls `decideResponderTransportMode`, `narrowChildSelectors`, builds mode from `sa.UseTransportMode`, echoes on it. The initiator response path calls `recordInitiatorTransportMode` then `recordInitiatorSelectors(sa, ..., nil)` and builds mode from `sa.UseTransportMode`.
- [ ] `internal/component/ike/engine/child.go` - `createFirstChildSA` takes the mode from `sa.UseTransportMode` (IKE_AUTH carrier, reached from `initiatorFirstChildSA` in `established.go` and from `buildAuthResponse`).
- [ ] `internal/component/ike/engine/mobike.go` - `childTransportMode` = `wantsTransportMode || UseTransportMode`; `acceptMobikeUpdate` refuses with UNACCEPTABLE_ADDRESSES when `sa.UseTransportMode`. `rebind.go` reads `anchor.Mode` and `childTransportMode`.
- [ ] `internal/component/ike/engine/sa.go` - field declarations; `inheritSendPath`.

**Behavior to preserve:** (unless the user explicitly said to change it)
- IKE_AUTH (both roles, EAP included) and new-child CREATE_CHILD_SA substitute and hold transport mode to one address exactly as today whenever Ze accepts transport mode.
- A tunnel-mode exchange and a NAT-free path take the identity path (`TestTunnelModeSelectorsUnchangedWithNATDetected`, `real-nat-tunnel-control`).
- A pool-bound peer's transport request is declined and narrows in tunnel mode.
- `initiateChildRekey` sends USE_TRANSPORT_MODE for a transport SA; `applyChildRekeyResponse` refuses a tunnel answer to it (`errChildRekeyTransportDeclined`, `childRekeyFixTransport`).
- A transport-mode request on a tunnel-mode Child SA rekey is declined: tunnel replacement, no notify in the answer.
- The rekey floor is compared in the substituted space (`TestChildRekeyFloorComparedInSubstitutedSpace`).
- `dropForeignInnerOverlaps` and `installChildSA` claim judge the installed mode.
- `show vpn ipsec sa` keeps reporting `original-tsi`/`original-tsr` from `OriginalTSiAddr/OriginalTSrAddr`.

**Behavior to change:** (only what the user asked for)
- The substitution, the single-address filter and the one-pair refusal are gated on the mode the exchange's Child SA installs, passed explicitly, never on IKE SA flags.
- A Child SA rekey (either role) after an IKE SA rekey is judged in transport mode when its Child SA is transport.
- The rekey responder installs tunnel mode, without the notify, for a rekey request that carries no USE_TRANSPORT_MODE, whatever the retired SA ran (RFC 7296 Section 1.3.1).
- A responder that declines a non-pool transport request (operator configured tunnel) no longer substitutes or holds the proposal to one address: it narrows the real selectors (Section 2.23.1, the "undo" outcome).
- `acceptMobikeUpdate` refuses an address update when the session's installed Child SA runs transport mode, independent of any IKE SA rekey.
- `PeerRequestedTransport` is deleted; `UseTransportMode` is written and read only by the IKE_AUTH exchange.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An inbound CREATE_CHILD_SA request carrying N(REKEY_SA) on an IKE SA that is itself the product of an IKE SA rekey: `PeerSession.handleCreateChildSAOwned` (`inbound.go`) -> `respondChildRekey`.
- The response to Ze's own Child SA rekey on such an IKE SA: `handleCreateChildSAOwned` (response) -> `applyChildRekeyResponse`.
- An inbound INFORMATIONAL with UPDATE_SA_ADDRESSES: `acceptMobikeUpdate`.

### Transformation Path
1. Responder rekey: parse the request (notify -> `transportRequested`, TSi, TSr).
2. Compute the exchange's mode once: transport when `transportRequested` and the retired Child SA runs transport, else tunnel.
3. `narrowChildSelectorsIn(sa, tsi, tsr, floor, mode)`: substitution (mode transport and NAT verdict), lease, policy match, single-address filter (mode transport), foreign-overlap filter (mode), closed subset.
4. `newRekeyedChild` builds the replacement with that mode; the answer echoes USE_TRANSPORT_MODE exactly when the mode is transport.
5. Initiator rekey: `applyChildRekeyResponse` refuses a tunnel answer to a transport rekey, then `recordInitiatorSelectors(sa, tsi, tsr, floor, old.Mode)`: substitution and one-pair refusal gated on that mode.
6. Creation exchanges compute their mode from the decision functions' return value and pass it the same way; IKE_AUTH stores it in `sa.UseTransportMode` for `createFirstChildSA`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> engine | parsed CREATE_CHILD_SA payloads, existing decrypt path | No |
| Engine -> dataplane | `installChildTolerant` installs the replacement with its mode (XFRM state `mode transport` or tunnel) | No |
| IKE SA -> successor IKE SA | `inheritSendPath` copies the NAT verdict; the mode is read from the inherited Child SA, nothing new is copied | No |

### Integration Points
- `narrowChildSelectorsIn` - its existing `mode` argument becomes the gate for the substitution and the single-address filter.
- `recordInitiatorSelectors` - gains the same `mode` argument; three callers (`adoptAuthResponseNegotiation`, the new-child initiator in `create_child.go`, `applyChildRekeyResponse`).
- `decideResponderTransportMode`, `recordInitiatorTransportMode` - return the mode instead of writing the SA.
- `newRekeyedChild` - receives the negotiated mode instead of copying `old.Mode`.
- `PeerSession.getChildSA` - the MOBIKE refusal reads the installed Child SA's mode.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the substitution stays inside the two role entry points (`rfcgate-1b-rfc7296-pilot.md`) |
| No unintended coupling (components stay isolated) | Yes | engine-internal signatures only |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses the existing `mode` argument and `ChildSA.Mode`; deletes `transportSelectorsInPlay` and `PeerRequestedTransport` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no new copies; one value argument |
| Registration over hardcoding, outbound (`ai/rules/plugins.md`) | N-A | no registry touched |
| Registration over hardcoding, inbound (`ai/rules/principles.md`) | N-A | no central enumeration touched |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every writer of `UseTransportMode` sets it true only when `wantsTransportMode(sa)` | `decideResponderTransportMode` and `recordInitiatorTransportMode` (`transport_mode.go`) are the only writers outside tests | dropping the `UseTransportMode` disjunct from `childTransportMode` changes MOBIKE/rebind | grep of writers in Phase 3; the existing mobike and rebind tests stay green | unvalidated |
| A-2 | A session holds one live Child SA, so `getChildSA` answers the installed mode | `PeerSession.childSA` (`reconcile.go`), `createdChild` replaces nothing on a childless SA (`inbound.go`) | the MOBIKE refusal misses an additional transport Child SA | read `reconcile.go` and `inbound.go` in Phase 3 | unvalidated |
| A-3 | No existing test asserts substitution or single-address refusal for a DECLINED non-pool transport request | the only such site is `rfc7296_transport_test.go` (`wide` SA with `PeerRequestedTransport` and no decision), which models a request Ze accepts (`transportPeer()` config) | a test encodes the old request-driven gate | run the engine package after Phase 2; route any changed expectation to `ze-test-judge` | unvalidated |
| A-4 | strongSwan, configured `mode = transport`, sends USE_TRANSPORT_MODE in every Child SA rekey request | `initiateChildRekey` comment cites strongSwan 5.9.14 `child_create.c`; RFC 7296 Section 1.3.3 "Usually" | the tunnel-on-no-notify rule would downgrade a strongSwan transport SA at each rekey | the interop scenario asserts `mode transport` on Ze's inbound state AFTER the child rekey | unvalidated |
| A-5 | `swanctl --rekey --child <name>` makes strongSwan initiate the Child SA rekey on demand | strongSwan swanctl CLI; `swanRekeyIKE` already uses `--rekey --ike` (`helpers.go`) | the scenario needs a short lifetime instead | first run of the new scenario | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A changed signature misses a caller and a path reads a zero mode (tunnel) silently | a NAT transport unit test goes red on IKE_AUTH or new-child | every caller is a compile error after the signature change; `TestResponderSubstitutesTransportSelectorsBeforeNarrowing` and `TestInitiatorAdoptsSubstitutedTransportSelectorsBehindNAT` must stay green unchanged |
| R-2 | Migrating test setup off `PeerRequestedTransport` touches RFC-tagged tests (`rfc7296_transport_test.go`, and any tagged file in the migration list) | the edit hook or commit gate asks for an approval | run `ze-test-judge` by an independent agent for each tagged file whose setup changes; record `RFC-approved:` / `Weakening-approved:` lines; never change an assertion's expectation to fit |
| R-3 | Installing a tunnel replacement for a transport SA changes forward-policy and XFRM install shape | install error or traffic loss in the unit dataplane mock | the replacement goes through `installChildTolerant` like any tunnel Child SA; the unit test asserts the installed state mode and the answer's missing notify |
| R-4 | The interop scenario races strongSwan's IKE_REKEYED state (charon drops non-Delete requests, `swanRekeyIKE` comment) | the child rekey request is ignored | wait until `swanctl --list-sas` shows one IKE SA with the new SPIs before `--rekey --child` |
| R-5 | Another session edits `internal/component/ike/engine` concurrently | `git status` shows foreign hunks in a file this spec edits | commit own hunks with `hunks <path> <patch>` (`ai/rules/git-safety.md`); never take foreign hunks out of the tree |
| R-6 | The engine test package stops compiling on HEAD again (journal `commit-needs-an-uncommitted-symbol.md` history) | `go vet ./internal/component/ike/engine/` fails on a symbol this spec did not touch | name the red and its owner in the commit body (`ai/rules/pre-release.md`); run the new tests with `-run` once the package compiles; do not edit the foreign file |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | IKEv2 Child SA negotiation in transport mode (all exchanges), MOBIKE address updates |
| How is it reverted? | revert the implementation commit; the spec and journal rows stay |
| Who else touches this path? | `spec-ike-unbound-peer-inner-address` (closed, 47d8c98086) touched `narrowChildSelectorsIn` and `inner_selectors.go`; check `./le spec status` for open IKE specs before Phase 2 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer's CREATE_CHILD_SA rekey of a NATed transport Child SA, delivered to `PeerSession.handleCreateChildSAOwned` on the successor of a real IKE SA rekey (`initiateIKERekey` -> `respondIKERekey` -> swap via `adoptRekeyedIKESA`) | → | `respondChildRekey` -> `narrowChildSelectorsIn` (mode transport) -> `substituteResponderSelectors`, `keepSingleAddress` | `TestTransportChildRekeyAfterIKERekeySubstitutesBehindNAT/peer_rekeys` in `internal/component/ike/engine/rfc7296_transport_nat_rekey_test.go` |
| Ze's own CREATE_CHILD_SA rekey of a NATed transport Child SA on the successor of a real IKE SA rekey, answered by the peer's `respondChildRekey` and fed back through `handleCreateChildSAOwned` (response) | → | `applyChildRekeyResponse` -> `recordInitiatorSelectors` (mode transport) -> `substituteInitiatorSelectors` | `TestTransportChildRekeyAfterIKERekeySubstitutesBehindNAT/ze_rekeys` |
| strongSwan behind the lab NAT box rekeys the IKE SA, then the transport Child SA | → | the whole responder path above, over the wire | interop scenario `real-nat-transport-rekey-after-ike-rekey` |
| Ze initiator behind the lab NAT: strongSwan rekeys the IKE SA, then Ze's Child SA lifetime rekeys the transport Child SA | → | the whole initiator path above, over the wire | interop scenario `real-nat-transport-ze-initiator-rekey-after-ike-rekey` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IKE SA rekeyed (successor built by the real rekey producers); the successor SA's `UseTransportMode` is false (precondition asserted); the peer, behind a NAT, rekeys a transport-mode Child SA proposing its pre-NAT pair (the NAT test constants: TSi the client's real address, TSr the server's public address) | Ze answers (no error, no TS_UNACCEPTABLE); the answer's TSi/TSr are the substituted observed pair (client public, server real); the answer carries USE_TRANSPORT_MODE; the replacement Child SA has `Mode` transport and selectors equal to the substituted pair; the successor SA's `OriginalTSiAddr/OriginalTSrAddr` hold the pre-NAT addresses |
| AC-2 | Same as AC-1, but the peer proposes TSi wider than one host (a /24 around its address) | Ze refuses with TS_UNACCEPTABLE and installs nothing (no new state in the dataplane mock); the same proposal at /32 is accepted (positive polarity) |
| AC-3 | IKE SA rekeyed; Ze rekeys its NATed transport-mode Child SA; the peer's answer names the pair in the peer's address space | Ze substitutes the answer back into its own space, accepts it, installs a transport replacement with selectors equal to Ze's view (client real, server public); no `errTSWidened` |
| AC-4 | Same as AC-3, but the answer carries two pairs | `applyChildRekeyResponse` returns an error wrapping `errTSTransportPairs`; nothing is installed |
| AC-5 | Child SA rekey request with NO USE_TRANSPORT_MODE for a transport-mode Child SA (with or without a prior IKE SA rekey) | the replacement installs tunnel mode, the answer carries no USE_TRANSPORT_MODE, the selectors are narrowed unsubstituted (equal to the proposal intersected with policy) |
| AC-6 | Child SA rekey request WITH USE_TRANSPORT_MODE for a tunnel-mode Child SA, NAT detected | replacement tunnel, no notify in the answer, selectors unsubstituted (preserved behavior) |
| AC-7 | IKE_AUTH or new-child CREATE_CHILD_SA: peer requests transport, Ze's peer config is tunnel mode, NAT detected, proposal TSi is a /24 | Ze answers in tunnel mode: no notify, the /24 narrowed against policy and NOT replaced by the observed address, no TS_UNACCEPTABLE |
| AC-8 | IKE_AUTH and new-child creation, transport accepted, NAT detected | substitution and single-address behavior unchanged: the existing tests `TestResponderSubstitutesTransportSelectorsBeforeNarrowing`, `TestInitiatorAdoptsSubstitutedTransportSelectorsBehindNAT`, `TestTransportSelectorRoundTripAcrossTwoNATs`, `TestSubstitutionStillRefusesAWidenedAnswer` and the RFC7296-2.23.1 tagged tests pass with their assertions unchanged |
| AC-9 | IKE SA rekeyed; the session's installed Child SA runs transport; peer sends UPDATE_SA_ADDRESSES | `acceptMobikeUpdate` returns UNACCEPTABLE_ADDRESSES; with a tunnel Child SA it proceeds to the address checks as today |
| AC-10 | Source after the change | `PeerRequestedTransport` and `transportSelectorsInPlay` no longer exist; `UseTransportMode` has no reader outside the IKE_AUTH path (`createFirstChildSA`, `selectAuthChildSA` echo, `narrowChildSelectors` wrapper used by `selectAuthChildSA` and `responder_eap.go`, `adoptAuthResponseNegotiation`) and its doc comment says so |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a transport-mode IPsec peer behind a NAT with Ze as responder; IKE SA and Child SA rekey on their lifetimes | strongSwan -> NAT box -> Ze `respondIKERekey` -> `respondChildRekey` | `real-nat-transport-rekey-after-ike-rekey` |
| 2 | Runs Ze as the transport-mode initiator behind a NAT; both SAs rekey | Ze `applyIKERekeyResponse`/`respondIKERekey` -> `initiateChildRekey` -> `applyChildRekeyResponse` | `real-nat-transport-ze-initiator-rekey-after-ike-rekey` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTransportChildRekeyAfterIKERekeySubstitutesBehindNAT` (subtests `peer_rekeys`, `ze_rekeys`) | `internal/component/ike/engine/rfc7296_transport_nat_rekey_test.go` (new) | AC-1, AC-3: real IKE SA rekey then real Child SA rekey through `handleCreateChildSAOwned`; rigs `errLink` / `newRefusedRig` (`cp_test.go`) with the NAT verdict set on both original SAs and carried by `inheritSendPath`; NAT constants and helpers from `ts_nat_substitute_test.go` | |
| `TestTransportChildRekeyAfterIKERekeyHoldsOneAddress` | same file | AC-2, tagged RFC7296-2.23.1-1 positive (/32 accepted) and negative (/24 refused TS_UNACCEPTABLE, nothing installed); discrimination record via `./le rfc record` (break: drop the single-address gate in `narrowChildSelectorsIn`) | |
| `TestTransportChildRekeyAfterIKERekeyRefusesTwoPairAnswer` | same file | AC-4 | |
| `TestChildRekeyWithoutTransportNotifyInstallsTunnel` | `internal/component/ike/engine/child_rekey_transport_test.go` | AC-5 | |
| `TestTransportRequestOnTunnelChildRekeyIsDeclined` | `internal/component/ike/engine/child_rekey_transport_test.go` | AC-6 | |
| `TestDeclinedTransportRequestNarrowsRealSelectors` | `internal/component/ike/engine/ts_nat_substitute_test.go` | AC-7, both creation entry points (`selectAuthChildSA` path via `narrowChildSelectors` after the decision, and `respondNewChild`) | |
| `TestMobikeUpdateRefusedForTransportChildAfterIKERekey` | `internal/component/ike/engine/mobike_test.go` | AC-9, both polarities | |
| existing NAT and transport tests | `ts_nat_substitute_test.go`, `rfc7296_transport_test.go`, `rfc7296_transport_nat_test.go`, `rfc7296_transport_one_pair_test.go`, `child_rekey_transport_test.go`, `rfc3948_inner_selector_ordered_transport_test.go` | AC-8: unchanged assertions; setup migrated from the deleted field to the decision functions' return value or the new `mode` argument | |

Test-setup migration list (setup only, never an expectation): `cp_refusal_fix_test.go`, `rfc3948_inner_selector_test.go`, `child_test.go`, `cp_test.go`, `ts_nat_substitute_test.go`, `rfc7296_transport_test.go` (sites that set `PeerRequestedTransport` then call `decideResponderTransportMode`), and every caller of `recordInitiatorSelectors` in tests. `TestOrderedTransportChildRekeyAfterIKERekey` keeps its hand-set `b.UseTransportMode = false`, which now proves the field is not read by the rekey. Any tagged file among them goes to `ze-test-judge` (R-2).

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Transport selector width (prefix length, IPv4) | 32 only | /32 (AC-2 positive) | /31 and wider refused (AC-2 /24) | N-A |
| Transport answer pair count | 1 only | 1 pair (AC-3) | 0 pairs refused by existing `closedSubset` check | 2 pairs refused (AC-4) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | `test/ipsec/*.ci` | No `.ci` IPsec test places an address translation between peers (none of the 34 files carries a NAT verdict), and the defect needs one. The end-to-end proof is the interop pair below, which runs a real NAT box | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `real-nat-transport-rekey-after-ike-rekey` | `test/interop-ipsec/scenarios/` (fixtures from `real-nat-transport-ze-responder`, Child SA `rekey_time = 0` on strongSwan) | strongSwan | Ze responder: after `swanRekeyIKE` and `swanctl --rekey --child ze-child`, the Child SA SPIs changed, Ze's selectors are still `natTransportZeLocal/natTransportZeRemote`, Ze's inbound state is `mode transport`, ESP crosses both directions. Red before the fix: TS_UNACCEPTABLE on the child rekey | |
| `real-nat-transport-ze-initiator-rekey-after-ike-rekey` | `test/interop-ipsec/scenarios/` (fixtures from `real-nat-transport-ze-initiator`, short Ze esp-group lifetime) | strongSwan | Ze initiator: after `swanRekeyIKE`, Ze's lifetime-driven Child SA rekey succeeds with the same four assertions. Red before the fix: the answer refused as widened | |

Both checkers register in `internal/le/interoplab/ipsec/checkers.go` and reuse `checkRealNATTransport`'s assertions. The red phase is recorded per `ai/rules/interop-and-goal-validation.md`: revert the engine change, rebuild the Ze image, observe red, restore, observe green. AC-5 has no interop scenario: a strongSwan transport child always sends the notify (A-4), so only the unit test can send a tunnel rekey for a transport SA.

## Files to Modify
- `internal/component/ike/engine/ts_nat_substitute.go` - delete `transportSelectorsInPlay`; both substitution producers take the exchange's mode and gate on it; comments rewritten (Section 2.23.1 quotes kept byte-exact)
- `internal/component/ike/engine/ts_narrow.go` - `narrowChildSelectorsIn` gates substitution and `keepSingleAddress`/`oneTransportPair` on `mode`; `recordInitiatorSelectors` gains `mode` and gates substitution and the one-pair refusal on it; `decidedChildMode` comment scoped to IKE_AUTH
- `internal/component/ike/engine/rekey.go` - `respondChildRekey` computes the negotiated mode once (transport iff request notify and retired SA transport), passes it to narrowing, `newRekeyedChild` and the echo; `newRekeyedChild` takes the mode; `applyChildRekeyResponse` passes `old.Mode` to `recordInitiatorSelectors`; stale comment at the `narrowChildSelectorsIn` call rewritten
- `internal/component/ike/engine/transport_mode.go` - `decideResponderTransportMode(sa, requested)` returns the mode; `recordInitiatorTransportMode` returns the mode and the delete verdict; `adoptAuthResponseNegotiation` stores the IKE_AUTH mode and passes it to `recordInitiatorSelectors`
- `internal/component/ike/engine/responder.go` - local `transportRequested` in `handleAuthRequest`; the decision's return value stored in `sa.UseTransportMode`
- `internal/component/ike/engine/create_child.go` - `respondNewChild` and the new-child initiator response use a local mode and call `narrowChildSelectorsIn` / `recordInitiatorSelectors` with it; no SA flag written
- `internal/component/ike/engine/sa.go` - delete `PeerRequestedTransport`; `UseTransportMode` doc comment: the IKE_AUTH exchange's negotiated mode, input to the first Child SA only, never the mode of an installed Child SA (`ChildSA.Mode`)
- `internal/component/ike/engine/mobike.go` - `acceptMobikeUpdate` reads the installed Child SA's mode via `getChildSA`; `childTransportMode` reads `wantsTransportMode` only (A-1)
- `internal/component/ike/engine/child.go` - comment at the `createFirstChildSA` mode read names the IKE_AUTH scope
- tests in the migration list above
- `internal/le/interoplab/ipsec/checkers.go`, `internal/le/interoplab/ipsec/helpers.go` - two checkers, a `swanRekeyChild` helper, a wait for the post-rekey IKE SA
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - field scope and MOBIKE refusal rule
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - rekey mode paragraph (line ~738) and the cross-peer filter paragraph (lines ~514-517, which names `sa.UseTransportMode` and the retired mode)
- pages the anchor command lists as mentioning this code (`ipsec-13-rekey-wire.md`, `ipsec-9-ikev2-eap-nat.md`, `docs/guide/ipsec.md`, `ipsec-11-interop-eap.md`, `ipsec-3-data-model.md`, `ipsec-6-ikev2-crypto.md`, `ipsec-dataplane-inspection.md`): each is read in the phase that changes its file and edited where it states the old mode rule; on 2026-10-27 none of them names `UseTransportMode`, `PeerRequestedTransport` or `transportSelectorsInPlay`
- `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - the gate definition
- `docs/architecture/ike/ipsec-14-responder.md` - pool-bound decline paragraph and its `transportSelectorsInPlay` source anchor
- `docs/architecture/testing/interop.md` or the IPsec scenario list it points at, if it enumerates scenarios

## Files to Create
- `internal/component/ike/engine/rfc7296_transport_nat_rekey_test.go` - wiring and AC-1..AC-4 tests
- `test/interop-ipsec/scenarios/real-nat-transport-rekey-after-ike-rekey/` - `nat.conf`, `swanctl.conf`, `ze.conf`
- `test/interop-ipsec/scenarios/real-nat-transport-ze-initiator-rekey-after-ike-rekey/` - `nat.conf`, `swanctl.conf`, `ze.conf`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config change |
| YANG validation constraints | No | no config change |
| YANG custom validators | No | no config change |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | No | no command added |
| Editor autocomplete | No | none |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output change |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | No | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | IKE, not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | a defect fix in an existing capability; evidence gains two interop scenarios, listed where the IPsec scenarios are listed |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | no operator-visible knob changes |
| 7 | Wire format changed? | Yes | the rekey answer's USE_TRANSPORT_MODE presence for a no-notify request: `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md`, `ipsec-8-ikev2-child-xfrm.md`; RFC7296-2.23.1-1 gains a tagged rekey-path test and its discrimination record |
| 10 | Test infrastructure changed? | Yes | two interop scenarios and a lab helper; scenario list in `docs/architecture/testing/interop.md` if it names IPsec scenarios |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-7-ikev2-engine.md` (mode state ownership, MOBIKE) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/release/fixes/spec-ike-transport-rekey-nat-state.md`; each page it names is edited in the phase that changes the file |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write the failing tests
   - Tests: `TestTransportChildRekeyAfterIKERekeySubstitutesBehindNAT`, `TestTransportChildRekeyAfterIKERekeyHoldsOneAddress`, `TestTransportChildRekeyAfterIKERekeyRefusesTwoPairAnswer`, `TestChildRekeyWithoutTransportNotifyInstallsTunnel`, `TestDeclinedTransportRequestNarrowsRealSelectors`, `TestMobikeUpdateRefusedForTransportChildAfterIKERekey`
   - Files: `rfc7296_transport_nat_rekey_test.go`, `child_rekey_transport_test.go`, `ts_nat_substitute_test.go`, `mobike_test.go`
   - Verify: each fails on HEAD for the stated reason (AC-1 TS_UNACCEPTABLE; AC-3 `errTSWidened`; AC-5 transport replacement; AC-7 substituted /32; AC-9 accepted update). Run: `./le job run label ike-rekey-nat quiet command go test -count=1 -run 'TransportChildRekeyAfterIKERekey|ChildRekeyWithoutTransportNotify|DeclinedTransportRequest|MobikeUpdateRefusedForTransport' ./internal/component/ike/engine/`
2. **Phase: Gate on the exchange's mode** -- `ts_nat_substitute.go`, `ts_narrow.go`, `rekey.go`, `create_child.go`, `transport_mode.go`
   - Delete `transportSelectorsInPlay` first, then add the `mode` gate (no hybrid). Decision functions return the mode. `respondChildRekey` computes the negotiated mode; `newRekeyedChild` takes it.
   - Tests: Phase 1 tests for AC-1..AC-7 pass; AC-8 tests pass with unchanged assertions; `TestTransportRequestOnTunnelChildRekeyIsDeclined` passes
   - Verify: `./le job run label ike-engine quiet command go test -count=1 ./internal/component/ike/engine/`
3. **Phase: Delete the duplicate state** -- `sa.go`, `responder.go`, `mobike.go`, `child.go`, test-setup migration
   - Delete `PeerRequestedTransport`; scope `UseTransportMode`; MOBIKE reads `getChildSA`. Run `ze-test-judge` for every tagged test file whose setup changed.
   - Tests: `TestMobikeUpdateRefusedForTransportChildAfterIKERekey`; existing `mobike_test.go`, `rfc3948_rebind_test.go`
   - Verify: the engine package run above; `grep` proves AC-10
4. **Phase: Interop, RFC record, docs**
   - Two scenarios and checkers; red/green walk per interop rule; `./le rfc record` for the RFC7296-2.23.1-1 tag in `TestTransportChildRekeyAfterIKERekeyHoldsOneAddress`; the three architecture pages; close journal rows (`one-state-held-in-two-fields.md` 2026-10-26, `derived-flags-overwrite-what-the-wire-carried.md` 2026-10-27).
   - Verify: each scenario by name through `./le test integration interop-ipsec` with its scenario selector (scoped run, never the full suite on the Linux host); `./le rfc check`

Gates owed by the main thread after the commits: `./le verify changed`, a scoped lint of `internal/component/ike/engine` and `internal/le/interoplab/ipsec`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | no remaining reader of `UseTransportMode` outside the IKE_AUTH path; every `recordInitiatorSelectors` / `narrowChildSelectorsIn` caller passes the mode its Child SA installs |
| Correctness | the respondChildRekey answer's notify, the replacement's `Mode`, and the narrowing gate all derive from the one computed value |
| RFC | quotes byte-exact; new implementing statements carry `// RFC 7296 Section X.Y: "..."`, callers carry the section reference (`ai/rules/rfc-compliance.md`, functions added from 2026-09-24) |
| Stale comments | the comments on `transportSelectorsInPlay`, `decidedChildMode`, the `narrowChildSelectorsIn` call in `respondChildRekey`, `keepSingleAddress` gate, `sa.go` fields and `childTransportMode` match the new behavior |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `transportSelectorsInPlay` and `PeerRequestedTransport` gone | `grep -rn 'transportSelectorsInPlay\|PeerRequestedTransport' internal/` empty |
| new tests pass | the Phase 2 package run |
| two scenarios registered | `grep -n 'rekey-after-ike-rekey' internal/le/interoplab/ipsec/checkers.go` |
| discrimination record | `rfc/discrimination/rfc7296.json` names `TestTransportChildRekeyAfterIKERekeyHoldsOneAddress` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | peer-proposed traffic selectors are untrusted input: the substitution replaces the peer's address with an OBSERVED one (`observedRemoteAddress`, `observedLocalAddress`), never with a peer assertion |
| Mode downgrade | a peer can only obtain tunnel for a rekey by asking for it (RFC 7296 Section 1.3.1); it cannot obtain transport on a tunnel SA; the selectors are still narrowed against policy and the floor |
| MOBIKE | the refusal must not be bypassable by an IKE SA rekey |

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

## Design Insights
- The defect class is a per-exchange negotiation outcome stored on a longer-lived object (the IKE SA) and read by later exchanges. The cure is to make the outcome a value of the exchange and keep the long-lived fact on the object that owns it (`ChildSA.Mode`).
- The IKE_AUTH exchange legitimately spans functions (and EAP rounds) before its Child SA exists, so it keeps an SA carrier, exactly as `NegotiatedPairs` does; that carrier is no longer read by any other exchange.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `one-state-held-in-two-fields.md` | 2026-10-26 | IKE transport Child SA rekey after an IKE SA rekey: no RFC 7296 2.23.1 substitution or single-address check (`transportSelectorsInPlay` reads SA flags `respondChildRekey` never sets) |
| `derived-flags-overwrite-what-the-wire-carried.md` | 2026-10-27 | IKE Child SA rekey responder installs the retired SA's mode whatever the request carried (`respondChildRekey`, `newRekeyedChild`) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Pass the exchange's mode explicitly to the two role entry points | (a) copy `UseTransportMode`/`PeerRequestedTransport` into the successor SA in both IKE rekey producers; (b) set the flags in `respondChildRekey`/`applyChildRekeyResponse` from `old.Mode` | (a) and (b) keep two sources and make the IKE SA flag a third writer's state; a later new-child exchange would still overwrite it. The explicit value has one source per exchange and the compiler finds every caller |
| Delete `PeerRequestedTransport`; keep `UseTransportMode` as the IKE_AUTH carrier only | delete both and thread the IKE_AUTH mode through `fsm.go` -> `established.go` and `handleAuthRequest` -> `buildAuthResponse` -> `selectAuthChildSA` / `responder_eap.go` | the request is read and decided inside one function, so it is a local; the IKE_AUTH decision crosses functions and EAP rounds before its Child SA exists, the same reason `NegotiatedPairs` lives on the SA. Removing the field would add a renamed carrier, not remove state |
| Responder gate = the decided mode, not the request | keep the request-driven gate (substitute whenever the peer asked) | RFC 7296 Section 2.23.1: when the SPD entry "does not allow transport mode", the server MAY undo the substitution and "will create an SA in tunnel mode using real Traffic Selectors". Ze knows its policy before the lookup (`decideResponderTransportMode`), so the declined request narrows the real selectors directly; the pool-bound exception becomes a case of the rule |
| Rekey with no USE_TRANSPORT_MODE installs tunnel | refuse the rekey (NO_PROPOSAL_CHOSEN); keep installing the retired mode | Section 1.3.1: "Except when using this option to negotiate transport mode, all Child SAs will use tunnel mode." Installing transport without the echo leaves the two ends on different modes; refusing leaves the SA to expire. Installing what the exchange negotiated matches the creation path, which already answers a tunnel request with tunnel |
| MOBIKE refusal reads the installed Child SA | read `wantsTransportMode` | the configuration can say transport while the peer negotiated tunnel; the installed Child SA is the fact RFC 4555's scope statement is about |

## Known Limitations
- `OriginalTSiAddr` / `OriginalTSrAddr` stay per IKE SA (written by the latest exchange that substituted); after an IKE SA rekey they are empty until the next Child SA rekey stores them. AC-1 asserts they are stored by that rekey. Moving them to the Child SA is not needed for any behavior Ze has (no production reader besides `show vpn ipsec sa`).

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

| Site | Quote (byte-exact from `rfc/full/rfc7296.txt`) |
|------|------------------------------------------------|
| the mode computation in `respondChildRekey` | Section 1.3.1: "Except when using this option to negotiate transport mode, all Child SAs will use tunnel mode." and Section 1.3.3 as quoted above |
| the mode gate in `narrowChildSelectorsIn` | Section 2.23.1: "In this case, the server should first check that the initiator requested transport mode, and then do address substitution on the Traffic Selectors." |
| the mode gate in `recordInitiatorSelectors` | Section 2.23.1: "If transport mode for the SA was selected (that is, if the server included USE_TRANSPORT_MODE notification in its response):" |
| the single-address filter (existing quote kept) | Section 2.23.1 MUST: "For transport mode, it MUST use exactly one IP address in the TSi and TSr payloads." |
| `acceptMobikeUpdate` refusal | RFC 4555 Section 1.1 (verified at `rfc/full/rfc4555.txt` line 129): "(transport mode IPsec SAs are beyond the scope of this document)." A scope statement, not a MUST: the comment cites it, no tag |

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits (`/ze-close` step 6f)
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
- [ ] **Commit B:** `remove plan/release/fixes/spec-ike-transport-rekey-nat-state.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

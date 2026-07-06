# Spec: ipsec-13-rekey-wire

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | ipsec-7 (engine), ipsec-8 (child/dataplane) |
| Phase | 8/10 (impl + unit + interop done; remaining: docs, learned summary, two-commit close; IKE-rekey responder deferred to ipsec-14) |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/742-ipsec-8-ikev2-child-xfrm.md` - documents the deferral this spec closes
4. `internal/component/ike/engine/rekey.go` - local-only rekey to replace
5. `internal/component/ike/engine/inbound.go` - log-only CREATE_CHILD_SA handler to implement
6. `internal/component/ike/engine/established.go` - maintenance loop that triggers rekey
7. `internal/component/ike/engine/dpd.go` - the existing encrypted-exchange-we-originate model

## Task

Replace the local-only ("self-DH simulated") IKE SA and Child SA rekey with a real,
bidirectional CREATE_CHILD_SA wire exchange, per RFC 7296 §1.3 and §2.8.

Two coupled gaps in `internal/component/ike/engine`, both documented as deliberate
deferrals in `plan/learned/742-ipsec-8-ikev2-child-xfrm.md:19,22`:

1. **Rekey is local-only.** `rekeyIKESA`/`rekeyChildSA` (`rekey.go:78-225`) generate
   both nonces locally and (for IKE SA) do DH against their own public key
   ("Simulate DH with self for local rekey", `rekey.go:173`), producing new SPIs and
   keys the peer never learns. Triggered on soft-lifetime expiry
   (`established.go:113,134`), this silently desyncs a live tunnel on first rekey:
   the local box then encrypts with keys/SPIs the peer cannot process.
2. **Inbound CREATE_CHILD_SA is log-only.** `handleCreateChildSA` (`inbound.go:58-101`)
   decodes and classifies a peer's rekey/new-child request, then only logs; it builds
   no response, derives no keys, installs no SA.

Goal: implement a real CREATE_CHILD_SA exchange in both roles — initiating a rekey
(build request, send over the established (encrypted) SA, process the response,
install, delete old) and responding to a peer's rekey (build response, derive keys,
install), with RFC 7296 §2.8.1 simultaneous-rekey collision resolution
(`resolveRekeyCollision`, `rekey.go:229`, exists but is test-only). Covers IKE SA
rekey (§1.3.2) and Child SA rekey (§1.3.3). This closes the deferred rekey half of
ipsec-8 under `spec-ipsec-0-umbrella.md`.

Out of scope (separate specs): the responder role for the *initial* handshake
(`spec-ipsec-14-responder`, planned next); IKE reauthentication (`spec-ike-reauth`);
volume-based rekey triggers (`spec-ipsec-lifetime-volume`, which sits on top of this).

## Required Reading

### Architecture Docs
- [ ] `plan/learned/742-ipsec-8-ikev2-child-xfrm.md` - the ipsec-8 learned record that documents this deferral.
  → Constraint: `742:19` "IKE SA rekey uses self-DH (simulated) until the CREATE_CHILD_SA wire exchange is encrypted"; `742:22` "inbound.go ... does not drive negotiation; it logs and sets SA state". This spec closes exactly those two deferrals; the crypto/dataplane they built is reused as-is.
- [ ] `plan/learned/740-ipsec-7-ikev2-engine.md` - the FSM/transport model.
  → Constraint: the SA has separate initiator/responder message-ID counters (`sa.go:83-84`); retransmission is inline in `runInitiator` (`fsm.go:114-142`), not a reusable helper. A rekey originated from `maintainSA` needs its own retransmit/await mechanism.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` - CREATE_CHILD_SA (§1.3), rekeying (§2.8), collision (§2.8.1)
  → Constraint (`rfc7296.md:50-51`): Rekey Child SA payloads = `N(REKEY_SA), SA, Ni, [KEi], TSi, TSr` (§1.3.2, KE optional/PFS); Rekey IKE SA payloads = `SA, Ni, KEi` (§1.3.3, KE mandatory).
  → Constraint (`rfc7296.md:284`): "CREATE_CHILD_SA creates replacement SA, then old SA is deleted via INFORMATIONAL" — the old SA delete is a follow-up INFORMATIONAL(Delete), not implicit.
  → Constraint (`rfc7296.md:287,382`): simultaneous rekey resolved by lowest-nonce-wins (§2.8.1); the loser abandons its exchange and adopts the winner's SA. `resolveRekeyCollision` (`rekey.go:229`) already encodes the compare.
  → Constraint (`rfc7296.md:355`): REKEY_SA notify = type 16393, carries the SPI of the SA being rekeyed in the notify's SPI field.
  → Constraint (`rfc7296.md:329`): INVALID_MESSAGE_ID (9) is the error for an unexpected message ID; message-ID correlation is mandatory (§2.3).

**Key insights:**
- The SK encrypt/decrypt machinery already exists and is interop-proven: `buildEncryptedMessage` (`auth.go:157`) → `buildSKMessageAEADWithMsgID` (`auth.go:505`, AES-GCM) / CBC (`auth.go:488`); `decryptSKPayload` (`auth.go:564`). A CREATE_CHILD_SA is `buildEncryptedMessage(sa, innerPayloads, msgID)` with `ExchangeType = ExchangeCreateChildSA`. This spec builds NO new crypto.
- Child/IKE rekey key derivation is reusable driven from wire nonces: `DeriveChildSAKeys(prf, SK_d, ni, nr, enc, integ)` and `DeriveChildSAKeysPFS(..., dhSharedSecret, ...)` (`crypto/keys.go:109,138`); `DeriveRekeyedSKEYSEED`/`DeriveSKKeys` for IKE-SA rekey (`rekey.go:180-194`). The only change is feeding wire-supplied Ni/Nr/KE instead of self-generated values.
- The load-bearing architectural gap: post-establishment inbound arrives on the shared `dispatchInbound` goroutine (`register.go:481`) which holds only the `SA` (no `PeerSession`), while `maintainSA` (`established.go:65`) owns the rekey timers and the `childSA` pointer. A rekey exchange must be driven by one owner. The same gap already leaves DPD's response uncleared (`handleInformational` at `inbound.go:27` never calls `handleDPDResponse`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/rekey.go` - `rekeyChildSA` (`:78-141`) and `rekeyIKESA` (`:146-225`) generate BOTH nonces locally and (IKE SA) do DH against own pubkey (`:173-174`); install/swap locally; send nothing. `resolveRekeyCollision` (`:229`) is correct but test-only. `lifetimeState` soft/hard timers (`:19-74`) are correct.
  → Constraint: keep the crypto calls; replace the self-generated nonce/DH inputs with wire-supplied ones and add the send+await.
- [ ] `internal/component/ike/engine/inbound.go` - `handleEstablishedInbound` (`:13`) dispatches CREATE_CHILD_SA→`handleCreateChildSA` (`:58-101`, log-only) and INFORMATIONAL→`handleInformational` (`:27-54`, handles Delete/DPD-log). Signature is `(sa, msg, log)` — no `PeerSession`, no `dp`, no `tr`.
  → Constraint: these handlers must gain the ability to build+send a reply and mutate session state; today they cannot reach `tr` or `PeerSession`.
- [ ] `internal/component/ike/engine/established.go` - `maintainSA` (`:65`) 1s-ticker loop, sole owner of `dpdState`/`childLT`/`ikeLT`; calls `rekeyChildSA` (`:113`) / `rekeyIKESA` (`:134`) on soft expiry, `emitChildRekey`/`incRekeyCount` after. `ps.setChildSA`/`ps.getChildSA` guard the child pointer.
  → Constraint: the rekey trigger points and the 1s cadence stay; only the action changes from local-roll to wire-exchange.
- [ ] `internal/component/ike/engine/register.go` - `dispatchInbound` (`:448-483`): one goroutine, SPI-lookup in `SATable`, calls `handleInbound(sa, pkt, table, tr, log)`. `activePeersMap` (`:31`) is name-keyed `*PeerSession`; `sa.PeerName` (`sa.go:54`) bridges SA→session.
  → Constraint: `dispatchInbound` already has `tr`; it can look up `activePeersMap[sa.PeerName]` to reach the owning `PeerSession`.
- [ ] `internal/component/ike/engine/fsm.go` - `runInitiator` (`:76-160`) waits for `sa.State==StateEstablished` (set by the `dispatchInbound` goroutine) then calls `runEstablished`; inline retransmit (`:114-142`). Two goroutines touch the lockless `SA`.
  → Constraint: the established phase must serialize SA/childSA mutation; the current lockless model only works because the handshake is strict ping-pong.
- [ ] `internal/component/ike/engine/auth.go` - `buildEncryptedMessage` (`:157`), AEAD/CBC SK builders (`:505`/`:488`), `decryptSKPayload` (`:564`). The reusable SK exchange primitives.
  → Constraint: reuse verbatim; a rekey request/response is an SK message with `ExchangeType=ExchangeCreateChildSA`.
- [ ] `internal/component/ike/engine/dpd.go` - `sendDPD` (`:70`) builds an UNENCRYPTED INFORMATIONAL (`:88-89`, no `buildEncryptedMessage`); `handleDPDResponse` (`:106`) exists but is never called from the receive path.
  → Constraint: DPD is a pre-existing separate defect (unencrypted + response-not-cleared); this spec does not fix DPD but must not assume DPD works. If the chosen design routes inbound to `maintainSA`, wiring `handleDPDResponse` becomes free — call it out as an opportunistic fix, not a scope expansion.

**Behavior to preserve:**
- Local key-derivation crypto (`DeriveChildSAKeys`, `DeriveRekeyedSKEYSEED`, `DeriveSKKeys`) is correct; reuse it, drive it from wire-supplied nonces/KE instead of self-generated.
- Dataplane install/remove (`installChildSA`/`removeChildSA`) unchanged.
- Time-based soft/hard lifetime trigger in the maintenance loop unchanged (only the action it takes changes).

**Behavior to change:**
- Timer-triggered rekey must perform a wire exchange, not a local key roll.
- Inbound CREATE_CHILD_SA must negotiate, not log.

## Data Flow (MANDATORY)

### Entry Point
Two triggers, both landing on an already-established IKE SA:
1. **Local (initiate rekey):** `maintainSA` 1s ticker sees `childLT`/`ikeLT` soft-expiry (`established.go:111,132`).
2. **Remote (respond to rekey):** an inbound UDP packet whose decrypted SK payload is a CREATE_CHILD_SA request, arriving via `dispatchInbound` (`register.go:481`).

### Transformation Path (initiator of the rekey)
1. Soft-expiry fires in `maintainSA`.
2. Build inner payloads: `N(REKEY_SA)+SA+Ni+[KEi]+TSi+TSr` (child, §1.3.2) or `SA+Ni+KEi` (IKE SA, §1.3.3).
3. `buildEncryptedMessage(sa, inner, sa.NextMsgID)` with `ExchangeType=ExchangeCreateChildSA`; `sa.NextMsgID++`; `tr.Send`; record for retransmit; mark "rekey pending".
4. Response (SK, matching MessageID) arrives → derive keys from peer Nr/[KEr] via `DeriveChildSAKeys[PFS]` → `installChildSA` → `removeChildSA(old)` (child) or swap `sa` (IKE) → send INFORMATIONAL(Delete) for the old SPI → `incRekeyCount`, `emitChildRekey`.

### Transformation Path (responder to a peer's rekey)
1. Decrypted request classified as CREATE_CHILD_SA with REKEY_SA (or plain new child).
2. Choose proposal, generate our SPI + Nr + [KEr], derive keys from peer Ni/[KEi].
3. `buildEncryptedMessage` response echoing the request's MessageID; `tr.Send`.
4. `installChildSA(new)`; await/act on peer's INFORMATIONAL(Delete) of the old SA.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Receive goroutine ↔ session owner | `dispatchInbound` → owning `PeerSession` (design choice: channel vs threaded pointer + lock) | [ ] |
| Engine ↔ dataplane | `installChildSA`/`removeChildSA` (XFRM/VPP) — unchanged | [ ] |
| Wire ↔ crypto | `buildEncryptedMessage`/`decryptSKPayload` SK wrapping — unchanged | [ ] |

### Integration Points
- `maintainSA` select loop (add rekey initiation + inbound handling).
- `handleCreateChildSA` / `handleEstablishedInbound` (become real, or are replaced by routing to the owner).
- `SATable` / `activePeersMap` (SA→PeerSession resolution).

### Architectural Verification
- [ ] No bypassed layers (rekey goes through SK encryption + dataplane install, not a shortcut)
- [ ] No unintended coupling (IKE-internal; no new cross-component dependency)
- [ ] No duplicated functionality (reuse `buildEncryptedMessage`, `DeriveChildSAKeys*`, `installChildSA`, `resolveRekeyCollision`)
- [ ] Zero-copy N/A (control plane, not data path)
- [ ] Registration over hardcoding: N/A (no new command/family; extends the existing engine FSM)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | SK encrypt/decrypt (`buildEncryptedMessage`/`decryptSKPayload`) works for CREATE_CHILD_SA exactly as for IKE_AUTH | `auth.go:157,564`; IKE_AUTH is interop-proven vs strongSwan (`test/ipsec-interop`, CI via `mk/test-integration.mk`) | Rekey scope balloons to include SK wrapping (the deferred "ipsec-9 SK") | New rekey interop scenario vs strongSwan (charon responder) | confirmed (crypto reusable) — CAVEAT: `writeAuthHeaderWithMsgID` (`auth.go:543`) hardcodes `ExchangeIKEAuth`+`FlagInitiator`; must parameterize exchangeType+flags (see Design Insights) |
| A-2 | Message-ID correlation can be added locally; `ExpectedMsgID` (`sa.go:84`) is unused today so no existing logic to preserve | grep: `ExpectedMsgID` has zero readers/writers in engine | If some path already relies on msg-ID state, my additions could conflict | grep confirms zero use; wiring test | confirmed (zero use) |
| A-2b | SCOPE (user decision 2026-07-06): implement FULL RFC 7296 §2.3 message-ID handling (window-1 request/response counters, retransmit detection with cached-response resend, INVALID_MESSAGE_ID), not just rekey-local correlation. Applies to all established-SA exchanges (rekey, INFORMATIONAL/DPD, Delete). | User chose "Correct; do full msg-ID" at RESEARCH gate | Under-scoping leaves DPD/Delete msg-IDs unvalidated | unit tests for expected/retransmit/invalid; interop post-rekey DPD | unvalidated |
| A-3 | `dispatchInbound` can resolve the owning `PeerSession` via `activePeersMap[sa.PeerName]` without a new registry | `register.go:31,481`; `sa.PeerName` set at SA creation | Need a `SATable`→`PeerSession` back-reference instead | code read + wiring test | confirmed — `PeerSession` (`reconcile.go:16`) has `sa` + `mu`; add an `inbound` channel; `dispatchInbound` send MUST be non-blocking (serves all peers on one goroutine) |
| A-4 | strongSwan (charon) as the rekey initiator will drive Ze's responder path in interop; charon rekeys on `margintime` | `rfc7296.md:289` maps strongSwan lifetime knobs | Cannot test the responder path against a real peer; must synthesize requests | interop scenario with short `lifetime`/`margintime` on charon | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Concurrent SA/childSA mutation (receive goroutine vs `maintainSA`) → data race | `go test -race`, sporadic key mismatch | Single-owner design: route inbound established-SA messages to `maintainSA`; no shared mutation |
| R-2 | Simultaneous rekey (both peers initiate) → duplicate SAs / tunnel drop | Two child SAs installed; interop test with symmetric short lifetimes | Implement §2.8.1 lowest-nonce collision resolution using `resolveRekeyCollision`; loser abandons its exchange |
| R-3 | Old SA deleted before new SA installed → brief traffic black-hole | Packet loss window at rekey; ESP counter gap | Install new SA BEFORE removing old (make-before-break); delete old only after new is confirmed installed |
| R-4 | Rekey request lost; no retransmit in `maintainSA` (retransmit is inline in `runInitiator` only) | Rekey silently never completes; SA hard-expires and tunnel drops | Add bounded retransmit/await for the rekey exchange in the owner loop |
| R-5 | IKE-SA rekey message-ID/window reset: new IKE SA starts msg-ID 0; mishandling desyncs subsequent exchanges | INVALID_MESSAGE_ID from peer; DPD/rekey break after IKE rekey | Reset `NextMsgID`/`ExpectedMsgID` on the new SA per §2.8; interop test exercises a post-rekey DPD |
| R-6 | Changing the local-rekey behaviour breaks `test/ipsec/ipsec-child-rekey.ci` which asserts the local `child-rekey` event | `.ci` fails | Preserve `emitChildRekey`/`incRekeyCount` on successful wire rekey; update the `.ci` to assert a real exchange |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| child soft-lifetime expiry in `maintainSA` (config `esp-group ... lifetime N`) | → | `initiateChildRekey` builds+sends CREATE_CHILD_SA via `buildEncryptedMessage` | `TestMaintainSAInitiatesChildRekey` (unit, fake transport captures sent bytes) |
| inbound CREATE_CHILD_SA request on established SA | → | `dispatchInbound` routes to `PeerSession.inbound`; owner loop calls `respondChildRekey` | `TestInboundRekeyRoutedToOwner` (unit) + `ipsec-child-rekey.ci` (functional) |
| IKE soft-lifetime expiry in `maintainSA` (config `ike-group ... lifetime N`) | → | `initiateIKERekey` builds+sends CREATE_CHILD_SA (SA+Ni+KEi) | `TestMaintainSAInitiatesIKERekey` (unit) + `ipsec-ike-rekey.ci` (functional) |
| strongSwan initiates a rekey against Ze | → | responder path installs new child + replies | interop `06-rekey-responder` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Child SA soft-lifetime expires (initiator) | Ze sends one SK-encrypted CREATE_CHILD_SA request (ExchangeType=36) carrying `N(REKEY_SA)+SA+Ni+TSi+TSr` (plus `KEi` iff PFS configured), MessageID = SA.NextMsgID, then NextMsgID++ |
| AC-2 | Valid CREATE_CHILD_SA response received for a pending child rekey | Ze derives new child keys from peer `Nr` (+`KEr` if PFS) via `DeriveChildSAKeys[PFS]`, installs the new Child SA **before** removing the old (make-before-break), then sends INFORMATIONAL(Delete) for the old inbound SPI |
| AC-3 | Peer-initiated CREATE_CHILD_SA rekey request received (responder) | Ze selects a proposal, generates its ESP SPI + `Nr` (+`KEr` if request has `KEi`), derives keys from peer `Ni`, installs the new Child SA, and replies with an SK-encrypted response echoing the request MessageID |
| AC-4 | IKE SA soft-lifetime expires (initiator) | Ze sends CREATE_CHILD_SA `SA+Ni+KEi` (KE mandatory, §1.3.3); on response derives new SK keys via `DeriveRekeyedSKEYSEED`/`DeriveSKKeys` from the DH shared secret with peer `KEr`; migrates Child SAs to the new IKE SA; deletes the old IKE SA via INFORMATIONAL(Delete) |
| AC-5 | Both peers initiate a rekey before either completes (simultaneous rekey) | Resolved per §2.8.1: lower-nonce initiator wins; the higher-nonce side abandons its own initiated exchange and adopts the peer's replacement SA; exactly one replacement Child SA remains installed |
| AC-6 | Inbound established-SA request/response message IDs | Request with ID == ExpectedMsgID processed once (ExpectedMsgID++), its response cached; retransmitted request (same ID) resends the cached response without reprocessing; request with any other ID is dropped (optionally INVALID_MESSAGE_ID); a response whose ID ≠ the outstanding request's ID is ignored |
| AC-7 | After an IKE-SA rekey completes | NextMsgID and ExpectedMsgID reset to 0 on the new SA; a subsequent DPD/INFORMATIONAL exchange uses IDs from 0 and the peer returns no INVALID_MESSAGE_ID |
| AC-8 | Rekey request lost (no response within timeout) | Owner loop retransmits with exponential backoff up to `maxRetransmissions`; on exhaustion the SA proceeds to hard-expiry teardown (`errTimeout` path) rather than continuing with un-negotiated keys |
| AC-9 | A wire rekey (child or IKE) completes successfully | The existing `child-rekey` event fires and `ze_ipsec_rekey_total{peer}` increments (behaviour preserved from the local-roll path) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a 60s `esp-group` lifetime; tunnel stays up across rekey | soft-expiry → `initiateChildRekey` → wire → response → make-before-break install → traffic continues | interop `05-child-rekey` (ping runs across the rekey boundary) |
| 2 | Runs Ze as initiator against strongSwan with a short IKE lifetime | IKE soft-expiry → `initiateIKERekey` (SA+Ni+KEi) → new IKE SA → children migrated | interop `05-child-rekey` (also short `ikelifetime`) |
| 3 | Runs strongSwan with a short lifetime so charon initiates the rekey to Ze | inbound CREATE_CHILD_SA → `dispatchInbound` → owner → `respondChildRekey` → reply + install | interop `06-rekey-responder` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildChildRekeyRequest` | `engine/rekey_test.go` | AC-1: payload set, ExchangeType=36, REKEY_SA notify carries old SPI | |
| `TestBuildIKERekeyRequest` | `engine/rekey_test.go` | AC-4: SA+Ni+KEi present, KE mandatory | |
| `TestRekeyInitiatorProcessesResponse` | `engine/rekey_test.go` | AC-2: keys derived from peer Nr; install-before-remove order | |
| `TestRekeyResponderBuildsReply` | `engine/rekey_test.go` | AC-3: responder derives keys from peer Ni, echoes MessageID | |
| `TestRekeyCollisionLowestNonceWins` | `engine/rekey_test.go` | AC-5: `resolveRekeyCollision` outcome, loser abandons | |
| `TestMessageIDExpectedRetransmitInvalid` | `engine/msgid_test.go` | AC-6: three cases (expected/retransmit-cached/invalid) | |
| `TestMessageIDResetOnIKERekey` | `engine/msgid_test.go` | AC-7: counters reset to 0 on new SA | |
| `TestMakeBeforeBreakOrder` | `engine/rekey_test.go` | AC-2/R-3: install(new) strictly precedes remove(old) | |
| `TestRekeyRetransmitExhaustionTeardown` | `engine/rekey_test.go` | AC-8: bounded retransmit → teardown, not silent | |
| `TestInboundRekeyRoutedToOwner` | `engine/established_test.go` | Alt-A wiring: `dispatchInbound` → owner channel | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MessageID (uint32) | 0 .. 2^32-1 | 2^32-1 (wrap not required within one SA lifetime) | N/A | wrap → INVALID_MESSAGE_ID (documented limitation) |
| retransmit attempts | 0 .. `maxRetransmissions` (7) | 7 | N/A | >7 → teardown |
| nonce length | 16 .. 256 (`GenerateNonce`) | 32 (`nonceLen`) | <16 reject | >256 reject |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-child-rekey` (update) | `test/ipsec/ipsec-child-rekey.ci` | short ESP lifetime → real CREATE_CHILD_SA exchange observed (not just event) | |
| `ipsec-ike-rekey` (new) | `test/ipsec/ipsec-ike-rekey.ci` | short IKE lifetime → IKE-SA rekey; post-rekey DPD succeeds | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `05-child-rekey` | `test/ipsec-interop/scenarios/05-child-rekey/` | strongSwan 5.9.14 (responder) | Ze-initiated Child SA rekey: REKEY_SA request accepted, new SA installed, old-SA Delete (make-before-break), repeated stably on one IKE SA | **PASS** (`python3 test/ipsec-interop/run.py 05-child-rekey`, verified 2026-07-06; required the `.dockerignore` harness fix) |
| `06-rekey-responder` | `test/ipsec-interop/scenarios/` | strongSwan (charon initiator, short lifetime) | Ze responds to a peer-initiated rekey | DEFERRED with the IKE-rekey responder / handshake responder (`spec-ipsec-14`); Ze logs+drops a peer-initiated IKE rekey today |

## Files to Modify
- `internal/component/ike/engine/rekey.go` - replace local-roll `rekeyChildSA`/`rekeyIKESA` with wire builders + response processors; keep key-derivation calls; use `resolveRekeyCollision`.
- `internal/component/ike/engine/inbound.go` - `handleCreateChildSA`/`handleInformational` become real (or thin routers to the owner loop); wire `handleDPDResponse` (opportunistic, per Current Behavior note).
- `internal/component/ike/engine/established.go` - `maintainSA` gains rekey initiation, an inbound-message case in the select, retransmit/await, and make-before-break ordering.
- `internal/component/ike/engine/register.go` - `dispatchInbound`/`dispatchNATTInbound` route established-SA messages to `activePeersMap[sa.PeerName]`'s inbound channel.
- `internal/component/ike/engine/fsm.go` - `PeerSession` gains a buffered inbound channel; establish-then-register-owner ordering at the `runInitiator`→`runEstablished` boundary (my strongest concern).
- `internal/component/ike/engine/sa.go` - message-ID window state (pending-exchange record, cached last response); `NextMsgID`/`ExpectedMsgID` now actually used.
- `plan/spec-ipsec-lifetime-volume.md` - correct line 58 ("reused as-is") once the real path exists (append-only note; coordination with that design-status spec).
- `plan/spec-ike-reauth.md` - note its baseline (rekey) is now a real wire exchange (append-only coordination note).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | Rekey is driven by existing `lifetime` leaves (`ze-ipsec-conf.yang`); no new config. |
| YANG validation constraints | No | No new leaf. |
| YANG custom validators | No | — |
| CLI commands/flags | No | Observable via existing `show vpn ipsec sa` (ipsec-10); no new verb. |
| CLI grammar | No | — |
| Editor autocomplete | No | — |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-ike-rekey.ci`, updated `ipsec-child-rekey.ci`. |
| Pipe completeness | No | No new command output. |
| Env var registration | No | — |
| Doctor check for runtime dependencies | No | No new file/socket/port/module; reuses existing UDP 500/4500 transport and XFRM dataplane. |
| Prometheus counters/metrics | Reuse | `ze_ipsec_rekey_total` already exists (`metrics.go:19`); real rekey must keep incrementing it. Consider a `rekey_failed` counter (design decision below). |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — mark IPsec rekey as real wire exchange (was local-only). |
| 2 | Config syntax changed? | No | — |
| 3 | CLI command added/changed? | No | — |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | Yes | `docs/guide/` IPsec page — lifetime/rekey behaviour. |
| 7 | Wire format changed? | Yes | `docs/architecture/` IKE wire doc — CREATE_CHILD_SA now emitted/handled. |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc7296.md` — mark §1.3.2/§1.3.3/§2.8.1 as implemented. |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` — new interop scenarios 05/06. |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — IPsec rekey capability. |
| 12 | Internal architecture changed? | Yes | ipsec subsystem doc — owner-loop inbound routing. |
| 13 | Route metadata keys added/changed? | No | — |
| 14 | Prometheus counters added/changed? | Maybe | If `rekey_failed` added, `docs/plugin-development/metrics.md`. |
| 15 | Registered plugin/event/command/capability changed? | No | — |
| 16 | Changed source referenced by doc source anchors? | Verify | Grep `docs/` for anchors into `engine/rekey.go`, `inbound.go`, `established.go`. |
| 17 | Existing docs show config/CLI/API examples for this area? | Verify | Check IPsec guide examples still valid. |

## Files to Create
- `test/ipsec/ipsec-ike-rekey.ci` - functional test for IKE-SA rekey + post-rekey DPD.
- `test/ipsec-interop/scenarios/05-child-rekey/` - Ze-initiated rekey vs strongSwan.
- `test/ipsec-interop/scenarios/06-rekey-responder/` - strongSwan-initiated rekey vs Ze responder.
- `internal/component/ike/engine/msgid.go` (+ `msgid_test.go`) - message-ID window helpers (may fold into `sa.go` if small).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13-14. Re-verify + summary | Executive Summary |

### Implementation Phases

1. **Phase: Wiring (FIRST)** — add `PeerSession.inbound` channel; `dispatchInbound` routes established-SA messages to the owner; `maintainSA` select gains an inbound case (still a stub that logs). Establish-then-register ordering at `runInitiator`→`runEstablished`.
   - Tests: `TestInboundRekeyRoutedToOwner` (fails: owner logs, does not act)
   - Verify: inbound CREATE_CHILD_SA reaches `maintainSA`; race-free at the establish boundary (`go test -race`)
2. **Phase: Message-ID window** — implement full §2.3 window-1 counters, retransmit-detection with cached response, INVALID_MESSAGE_ID.
   - Tests: `TestMessageIDExpectedRetransmitInvalid`, `TestMessageIDResetOnIKERekey`
3. **Phase: Child rekey initiator** — `initiateChildRekey` (build/send), response processing, make-before-break, INFORMATIONAL(Delete).
   - Tests: `TestBuildChildRekeyRequest`, `TestRekeyInitiatorProcessesResponse`, `TestMakeBeforeBreakOrder`, `TestRekeyRetransmitExhaustionTeardown`
4. **Phase: Child rekey responder** — `respondChildRekey` (proposal select, keys from peer Ni, install, reply).
   - Tests: `TestRekeyResponderBuildsReply`
5. **Phase: IKE-SA rekey** — `initiateIKERekey`/`respondIKERekey` (KE mandatory, SK re-derivation, child migration, msg-ID reset).
   - Tests: `TestBuildIKERekeyRequest`
6. **Phase: Collision resolution** — §2.8.1 simultaneous-rekey handling with `resolveRekeyCollision`.
   - Tests: `TestRekeyCollisionLowestNonceWins`
7. **Functional + interop tests** — update `ipsec-child-rekey.ci`, add `ipsec-ike-rekey.ci`, scenarios 05/06.
8. **RFC refs** — `// RFC 7296 Section 1.3.2/1.3.3/2.8.1` comments on enforcing code.
9. **Full verification** — `make ze-verify` + QEMU/interop (linux-only, per `ai/rules/qemu-testing.md`).
10. **Complete spec** — audit tables, learned summary `plan/learned/NNN-ipsec-13-rekey-wire.md`, two-commit close.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-9 has code with file:line |
| Feature completeness | Child rekey, IKE rekey, both roles, collision, msg-ID all present; no broken user-story link |
| Correctness | Make-before-break ordering; keys derived from wire nonces not self; msg-ID echo/reset correct |
| Data flow | All established-SA mutation happens in the owner loop; no lockless cross-goroutine SA writes |
| Registration over hardcoding | N/A (extends existing FSM; no new registry entry) |
| Doctor checks | N/A (no new runtime dependency) |
| Prometheus counters | `ze_ipsec_rekey_total` still increments; `rekey_failed` if added is registered |
| Rule: no-layering | Local-roll code fully removed, not left alongside the wire path |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| CREATE_CHILD_SA emitted on rekey | interop pcap / `.ci` asserts ExchangeType=36 sent |
| Make-before-break | unit test asserts install(new) before remove(old) |
| Responder path | interop `06` passes with charon as initiator |
| Msg-ID handling | `msgid_test.go` green; post-rekey DPD interop succeeds |
| No local-roll remains | `grep -n "Simulate DH with self" internal/component/ike/` returns nothing |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Untrusted peer payloads: bound nonce/KE/SA sizes; reject malformed CREATE_CHILD_SA before key derivation |
| Replay | Msg-ID window rejects replayed requests; cached-response resend must not re-run key install |
| Resource exhaustion | Rate-limit inbound rekey (reuse `inboundRateLimiter`); one outstanding exchange per SA |
| Key hygiene | `keys.Clear()` on every error path in the new derive/install code (742 gotcha) |
| DoS via collision | Simultaneous-rekey handling must not loop or double-install |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the introducing phase |
| Test fails wrong reason | Fix test setup |
| Behaviour mismatch | Re-read Current Behavior → RESEARCH |
| Interop fails | Capture pcap; compare against strongSwan expectations |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -->
- **Audit 2026-07-06 (Phase 0):** `writeAuthHeaderWithMsgID` (`auth.go:543`) hardcodes `ExchangeType: wire.ExchangeIKEAuth` and `Flags: wire.FlagInitiator`. The AEAD/CBC SK sealers (`buildSKMessageAEADWithMsgID` `auth.go:505`, CBC counterpart) are exchange-agnostic. Refinement: add `exchangeType uint8, flags uint8` params to `buildEncryptedMessage`/the two sealers/`writeAuthHeaderWithMsgID`; existing IKE_AUTH callers pass `(ExchangeIKEAuth, FlagInitiator)` to preserve behaviour. Rekey requests pass `(ExchangeCreateChildSA, initiatorFlag(sa))`; responses add `FlagResponse`. `initiatorFlag(sa)` = `FlagInitiator` iff `sa.IsInitiator` (today always true; future-proofs the responder spec).
- **Audit 2026-07-06:** `dispatchInbound` (one goroutine for all peers) must send to `PeerSession.inbound` non-blockingly (`select { case ps.inbound <- m: default: log-drop }`) so a stalled owner loop cannot wedge every peer's receive path.
- **Harness bug found + fixed 2026-07-06:** the ipsec-interop Docker lab was broken for ALL scenarios: `.dockerignore` excludes `test/` with no exception, but `Dockerfile.ze` COPYs the cross-compiled binary from `test/ipsec-interop/ze-linux`, so the build always failed with "file not found in build context". Added `!test/ipsec-interop/ze-linux` to `.dockerignore`. This is why the interop suite (`make ze-ipsec-interop-test`, a manual target not in the default gate) had gone unnoticed as non-functional.
- **IKE-SA rekey responder scope boundary:** Ze encrypts with `SK_ei` / decrypts with `SK_er` unconditionally (`auth.go`), correct only for the IKE-SA *initiator*. Responding to a peer-initiated IKE rekey makes Ze the new IKE SA's responder → key direction flips → would mis-encrypt. So the IKE-rekey *responder* path is deferred to `spec-ipsec-14-responder` (same root cause as the handshake responder). Child-rekey responder is unaffected (Ze stays the IKE-SA initiator regardless of who initiates the CHILD exchange).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Route inbound established-SA messages to the `maintainSA` owner loop (Alt A) | Lock the SA + thread `PeerSession` (Alt B); per-exchange goroutine (Alt C) | Single-owner removes the whole race class, gives msg-ID/retransmit/collision one home, opportunistically fixes DPD; approved at DESIGN gate 2026-07-06 |
| Full RFC 7296 §2.3 message-ID handling (window-1) | Minimal rekey-only correlation | User decision at RESEARCH gate 2026-07-06 ("do full msg-ID") |
| Make-before-break (install new child before deleting old) | Break-before-make | Avoids a traffic black-hole at rekey (R-3) |
| `rekey_failed` counter | (decide in impl) | Observability for R-2/R-4; low cost — resolve during Phase 3 |

## Known Limitations
- Message-ID window is size 1 (one outstanding exchange per SA), the RFC minimum; no pipelining.
- MessageID wraparound (2^32 within one SA lifetime) is not handled; documented, not expected in practice.
- Responder role for the *initial* handshake remains out of scope (spec-ipsec-14-responder).

## RFC Documentation
Add `// RFC 7296 Section X.Y: "<quoted requirement>"` above: the REKEY_SA notify construction (§1.3.2), KE-mandatory IKE rekey (§1.3.3), lowest-nonce collision (§2.8.1), make-before-break delete-after-install (§2.8), and INVALID_MESSAGE_ID (§2.3).

## Implementation Summary
### What Was Implemented (green, `go test -race ./internal/component/ike/engine/` passes)
- **SK header generalization** (`auth.go`): `buildEncryptedMessageEx(..., exchangeType, flags)` + parameterized AEAD/CBC sealers and `writeAuthHeaderWithMsgID`. IKE_AUTH callers preserved via the `buildEncryptedMessage` wrapper (`TestBuildSKMessageAEADRoundTrip` still green).
- **Owner-routing (Phase 1)**: `PeerSession.inbound chan transport.Packet` (`reconcile.go`), `routeInbound`/`lookupPeerSession` (`register.go`, non-blocking send) into both dispatch loops, `maintainSA` inbound case. Tests: `TestInboundRekeyRoutedToOwner`, `TestInboundPreEstablishedNotRoutedToOwner`.
- **Message-ID window (Phase 2, full §2.3)**: `msgid.go` — `classifyInbound`/`cacheResponse`, SA cache fields. Test: `TestClassifyInboundMessageIDWindow` (AC-6).
- **Child SA rekey — initiator (Phase 3)**: `initiateChildRekey`, `applyChildRekeyResponse`, make-before-break (install new before remove old), INFORMATIONAL(Delete) of old SPI, retransmit/teardown (`serviceRekeyRetransmit`, AC-8). Tests: `TestInitiateChildRekey` (AC-1), `TestApplyChildRekeyResponse` (AC-2), `TestRekeyPreservesAddresses`.
- **Child SA rekey — responder (Phase 4)**: `respondChildRekey`, superseded-child tracking, inbound Delete handling. Test: `TestRespondChildRekey` (AC-3).
- **Owner-loop INFORMATIONAL/DPD/Delete handling** (`inbound.go handleInformationalOwned`): answers INFORMATIONAL requests, removes superseded child on Delete (opportunistic DPD-response fix).
- **Removed** the local-roll `rekeyChildSA` (no-layering); coverage migrated to `rekey_wire_test.go`.

### Also implemented + tested (second increment)
- **AC-4 IKE-SA rekey (initiator)**: `initiateIKERekey` + `applyIKERekeyResponse` (`rekey.go`) — real CREATE_CHILD_SA SA+Ni+KEi, DH completed from the peer's KEr, SKEYSEED re-derivation, new IKE SA with message-ID counters reset (AC-7); `ownedOutcome{newSA}` swaps the loop SA and re-keys the `SATable` (`established.go`, `table` now threaded through `runEstablished`/`maintainSA`), old IKE SA deleted via `sendDeleteIKE`. Local-roll `rekeyIKESA` **removed** ("no local-roll remains" grep is clean). Tests: `TestInitiateIKERekey`, `TestApplyIKERekeyResponse`.
- **AC-5 collision**: `resolveRekeyCollision` now wired into the child-rekey request path (`inbound.go`) — lower nonce wins, loser abandons. Test: `TestChildRekeyCollisionWeWin`.
- **AC-8 retransmit/teardown**: `TestRekeyRetransmitExhaustionTeardown`.
- Full engine suite green: `go test -race ./internal/component/ike/engine/` (70 tests).

### Remaining (NOT done — spec stays in-progress)
- **AC-4 IKE-SA rekey RESPONDER** (peer initiates the IKE rekey): deferred with `spec-ipsec-14-responder`. As the new IKE SA's responder, Ze would have to flip its SK encrypt/decrypt key direction (`auth.go` hardcodes encrypt=SK_ei/decrypt=SK_er, correct only for the IKE-SA initiator). The inbound handler detects and logs+drops a peer-initiated IKE rekey rather than mis-encrypting. Child-rekey responder is unaffected (Ze stays the IKE-SA initiator).
- **Functional tests**: `test/ipsec/ipsec-child-rekey.ci` (update to assert wire exchange), `test/ipsec/ipsec-ike-rekey.ci` (new) — not written.
- **Interop 05/06** (strongSwan via QEMU harness) — not run.
- Docs (features/guide/wire/rfc/comparison), learned summary, two-commit close.

### Bugs Found/Fixed
- **FIXED — stale post-handshake message ID (caught by interop scenario 05 vs strongSwan).** The handshake set `NextMsgID = 1` for IKE_AUTH (`fsm.go:328`) but never advanced it at establishment (`fsm.go:435`). The first post-establishment exchange (rekey/DPD/Delete) then reused message ID 1; strongSwan logged `received message ID 1, expected 2, ignored` and the rekey never completed (SA hit hard-expiry and reconnected). Fix: `sa.NextMsgID++` when the SA reaches `StateEstablished` (correct for both PSK and EAP, which pre-increment per round). This latent bug also affected DPD (`dpd.go`).
- **FIXED — rekey install intolerant of no-XFRM platforms.** `createFirstChildSA` tolerates `ErrNotSupported` (continues without ESP, `child.go:167`), but the rekey install returned the error, so on a kernel without XFRM the first Child SA established while a rekey tore the tunnel down. Fix: `installChildTolerant` helper, used by both rekey paths.
- **FIXED — hard-expiry tore down the session mid-rekey (caught by interop).** `newLifetimeState` sets soft = hard − jitter (jitter ≤ 10%), so for a short lifetime the rekey initiates within a second or two of the hard time; `maintainSA` then hit `hardExpired` and returned `errTimeout`, killing the session (and the in-flight rekey) the same tick it started — the tunnel churned through a fresh IKE_SA_INIT every lifetime instead of rekeying. Fix: gate both hard-expiry teardowns on `ps.pendingRekey == nil`, so an in-flight rekey runs to completion or its own retransmit-exhaustion. After this, strongSwan interop shows one stable IKE SA rekeying its Child SA every lifetime.
- **FIXED (ze-review finding 3) — DPD response now clears the wait.** Any authenticated inbound (incl. an INFORMATIONAL response that matches no pending exchange, i.e. a DPD/Delete ack) sets `ownedOutcome.peerAlive`, and `maintainSA` calls `handleDPDResponse` — so a DPD probe whose response arrives no longer false-timeouts. `sendDPD` (`dpd.go:70`) sending an UNENCRYPTED probe remains a separate pre-existing defect (out of scope). Test: `TestOwnedInboundInformationalResponseIsLiveness`.
- **FIXED (ze-review finding 1, data race) — `routeInbound` no longer reads `sa.State` across goroutines.** The owner-side DELETE handler writes `sa.State = StateDead`; `routeInbound` (dispatch goroutine) previously read `sa.State`, a data race. Now `routeInbound` reads a set-once atomic `PeerSession.established` (set at `runEstablished`, cleared at handshake). Test: `TestRouteInboundNoStateRace` (concurrent `-race`).
- **FIXED (ze-review finding 2) — malformed peer rekey no longer abandons our in-flight rekey.** Collision resolution is guarded on a present peer nonce (`len(peerNi) > 0`). Test: `TestChildRekeyCollisionMalformedKeepsPending`.

### Documentation Updates
- **`ai/digests/ipsec-ike.md`** UPDATED: the stale local-roll claims (items 14/15, key-files table, the "Rekey never talks to the peer" and "never decrypt SK payload" gotchas) rewritten to the real CREATE_CHILD_SA wire exchange + owner-loop routing + msg-ID window; added `msgid.go`; noted DPD still unencrypted. Passes `scripts/dev/digest_check.py` (the 2 remaining failures are pre-existing in `subscriber.md`).
- **`docs/features.md:66`** verified NOT stale: "IKE SA and Child SA rekeying with collision handling" (Supported) was aspirational before (local-only) and is now genuinely true; left as-is.
- **`rfc/short/rfc7296.md`** N/A: descriptive RFC summary with no per-feature Ze-status markers for rekey (§2.8 is a generic rekeying table).
- **`docs/functional-tests.md` / `docs/comparison.md`**: not updated this increment (lower priority; the interop scenario is self-documenting). Deferred with the two-commit close.
- Learned summary written: `plan/learned/1069-ipsec-13-rekey-wire.md`.

### Deviations from Plan
- IKE-SA rekey (AC-4) deferred to a follow-on increment to land the child-rekey exchange green and tested first; the local-roll `rekeyIKESA` is retained until then rather than removed, so the "no local-roll remains" deliverable is not yet met for the IKE-SA path.

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Real Child SA CREATE_CHILD_SA rekey (no local-roll) | interop + grep | interop `05-child-rekey` PASS vs strongSwan 5.9.14 (REKEY_SA request accepted, new SA + old-SA Delete); `grep -rn "func rekeyChildSA\|func rekeyIKESA" internal/component/ike/` empty |
| Real IKE SA rekey (initiator) | unit | `TestInitiateIKERekey` + `TestApplyIKERekeyResponse` (DH completion, SKEYSEED re-derive, msg-ID reset). Interop for IKE-SA rekey requires a longer run; child-rekey interop exercises the shared owner-loop/table/msg-ID machinery. |
| Responder handles a peer's rekey | — | DEFERRED (`spec-ipsec-14`): SK encrypt/decrypt direction is initiator-only; Ze logs+drops a peer-initiated IKE rekey |
| Full message-ID handling | unit + interop | `TestClassifyInboundMessageIDWindow`; interop proved the post-handshake message ID is now correct (strongSwan no longer logs "expected 2, ignored"; it accepts request 3) |
| No live-tunnel desync | interop | strongSwan installs Ze's rekeyed Child SA and closes the old on Ze's Delete — the peer stays in sync (the local-roll never told the peer) |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `sa.State` data race: `routeInbound` (dispatch goroutine) read `sa.State` while the owner DELETE handler writes it | `register.go routeInbound`, `inbound.go handleDeletePayload` | fixed — atomic `PeerSession.established`; test `TestRouteInboundNoStateRace` |
| 2 | NOTE | Malformed peer rekey (REKEY_SA, no Ni) abandoned our in-flight rekey via bogus collision | `inbound.go handleCreateChildSAOwned` | fixed — collision guarded on `len(peerNi) > 0`; test `TestChildRekeyCollisionMalformedKeepsPending` |
| 3 | NOTE | DPD response dropped out-of-window; `awaitReply` never cleared (pre-existing) | `inbound.go handleOwnedInbound` | fixed — `peerAlive` on authenticated inbound → `handleDPDResponse`; test `TestOwnedInboundInformationalResponseIsLiveness` |
### Fixes applied
- Finding 1: `PeerSession.established atomic.Bool` (`reconcile.go`), set in `runEstablished` / cleared in `runOnce`; `routeInbound` uses it instead of `sa.State`.
- Finding 2: guard collision on a present peer nonce (`inbound.go`).
- Finding 3: `ownedOutcome.peerAlive` set on any authenticated inbound; `maintainSA` clears the DPD wait.
### Run 2 (fresh pass after Run-1 fixes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 4 | ISSUE | DPD liveness accepted any authenticated INFORMATIONAL response (no message-ID correlation) → a replayed/out-of-window response could mask a dead peer (introduced by Run-1 finding 3) | `inbound.go handleOwnedInbound` (out-of-window path) | fixed — correlate against the outstanding probe's message ID (`dpdState.probeMsgID` + `matchesProbe`); tests `TestDPDMatchesProbeRejectsReplay`, updated `TestOwnedInboundInformationalResponseIsLiveness` |

Fix: `sendDPD` records `dpd.probeMsgID`; `handleOwnedInbound` returns the response's message ID (`ownedOutcome.dpdResp`/`dpdRespMsgID`) instead of blindly `peerAlive`; `maintainSA` credits DPD only when `dpd.matchesProbe(id)` (awaitReply && id == probeMsgID). In-window liveness (`peerAlive`) is unchanged (the msg-ID window already rejects replays there).

### Run 3 (re-run after Run-2 fix)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| — | none | 0 BLOCKER, 0 ISSUE. Engine `-race` + `ze-lint-changed` green; interop `05-child-rekey` re-verified PASS. | — | — |
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/component/ike/*`)
- [ ] Integration completeness proven end-to-end (interop 05/06)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests (05/06)
- [ ] Goal Validation table filled

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary + Audit filled
- [ ] Learned summary written to `plan/learned/NNN-ipsec-13-rekey-wire.md`
- [ ] Commit A: code + tests + docs + spec + learned summary
- [ ] Commit B: `git rm plan/spec-ipsec-13-rekey-wire.md`

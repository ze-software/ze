# 1072 -- IKE Responder Role (handshake + EAP-server + IKE-rekey responder)

## Context

Ze was initiator-only: `runResponder` blocked on `stopCh`, `dispatchInbound` dropped any
packet with no `SATable` entry, and `connection-type respond` was a silent black hole. The
ROOT cause shared with ipsec-13's deferred IKE-rekey responder was that message DIRECTION
(SK encrypt/decrypt keys, key-derivation nonce order, AUTH octets, ESP key roles) was
hardcoded to the initiator seat. `spec-ipsec-14-responder` implemented the responder handshake
(PSK/X.509 + EAP authenticator), the IKE-rekey responder (closing ipsec-13), and generalized
all direction sites by `sa.IsInitiator`. Builds on ipsec-13 (owner loop, `PeerSession.established`,
`buildEncryptedMessageEx`).

## Decisions

- **Direction is THREE concerns, parameterized by role, not one.** The spec assumed "SK crypto
  is the only hardcoded-direction piece (6 sites)". Audit found two more: key DERIVATION needs
  absolute Ni|Nr order (`sa.initiatorNonce()`/`responderNonce()`), and ESP install needs the
  send/recv KEYMAT half by role (`ChildSA.LocalIsInitiator`). AND `computeSignedOctets` (AUTH)
  was initiator-seat only. All generalized so an initiator SA is byte-identical (helpers reduce
  to Remote/Local when `IsInitiator=true`).
- **Mirror the initiator FSM (Alt A), not owner-loop the handshake.** The responder SA is created
  by the dispatch goroutine (`tryResponderSAInit` on an unsolicited IKE_SA_INIT request from a
  configured `respond` peer), advanced inline on that goroutine (`handleResponderInbound` →
  `handleSAInitRequest`/`handleAuthRequest`), and a polling `runResponder` adopts it into the
  owner loop at establishment. Same concurrency model as the initiator; `ps.sa` handoff guarded
  by `setSA`/`getSA`, one handshake per peer via `responderBusy` (AC-6).
- **Reuse the EAP server, don't reimplement.** `eap.Session` (Begin/Process/Succeeded/MSK) was
  fully implemented + unit-tested but had ZERO callers; the responder wires it via `NewEAPSession`.
  The responder authenticates itself with its long-term credential (`computeServerAuth` → cert or
  PSK) in the first IKE_AUTH, then runs EAP, then exchanges MSK-derived AUTH.
- **Responder child install is asymmetric.** The responder installs the first Child SA inside
  `handleAuthRequest` (it must reply with SAr2/TSr), so `runEstablished` ADOPTS `ps.getChildSA()`
  for `!IsInitiator` instead of calling `createFirstChildSA`.
- **IKE-rekey responder make-before-break.** `respondIKERekey` derives the new IKE SA (peer is the
  rekey initiator → new SA `IsInitiator=false`), replies under the OLD keys, holds the new SA in
  `ps.pendingIKESwap`, and swaps when the peer's INFORMATIONAL Delete of the old SA arrives.

## Consequences

- Interop-verified as responder vs strongSwan 5.9.14: `07-responder-psk` (strongSwan established
  IKE+Child SA against Ze the responder) and `09-responder-ike-rekey` (strongSwan "IKE_SA ze[2]
  rekeyed") PASS. macOS Docker has no XFRM for the Ze container, so responder interop is
  CONTROL-plane (strongSwan authoritative); dataplane gates on XFRM availability.
- `08-responder-eap-mschapv2` (cert-based, standard EAP) is blocked by a pre-existing Ze PKI-config
  limitation (base64-DER cert values + `private { key }` container expected, not `.pem` file paths
  / `private-key` field — scenario 04 initiator EAP-TLS hits the identical gap). The EAP responder
  is proven host-independently by `TestResponderEAPSessionWired`; the strongSwan cross-check awaits
  the separate PKI file-path-loading fix.
- Deterministic host-independent proof is the in-process end-to-end handshake tests
  (`responder_test.go`): full PSK handshake, full EAP-MSCHAPv2 handshake (client `eap.PeerSession`
  ↔ server `eap.Session` through the IKE layer), and IKE-rekey — all reaching established with
  matching keys/SPIs. All engine tests green under `-race` (responder handoff is race-free).
- Closes ipsec-13's deferred IKE-rekey responder.

## Gotchas (bugs found)

- **AUTH octets were initiator-seat only (`computeSignedOctets`).** It appended `sa.RemoteNonce`/
  `sa.LocalNonce` and picked the ID via `!isInitiator`; both invert for a responder SA (Local=Nr,
  Remote=Ni). RFC 7296 §2.15 wants Nr on the initiator octets, Ni on the responder octets, each
  party's own ID. Fixed to absolute nonces + `isInitiator != sa.IsInitiator`. The spec's A-3
  ("AUTH already fully direction-aware") was WRONG.
- **IKE identity type: IP literals must be ID_IPV4_ADDR, not ID_FQDN.** `buildIDPayload` always
  emitted ID_FQDN. Masked in initiator scenarios because the peer had no id constraint; strongSwan
  as the constrained initiator rejected Ze's responder IDr with "constraint check failed: identity
  '172.28.0.2' (ID_IPV4_ADDR) required, not matched by ... (ID_FQDN)". Fixed with `encodeIKEID`
  (net.ParseIP → address type). This fix was found ONLY by the interop; the in-process tests use a
  non-IP PeerName and passed regardless.
- **Latent `respondChildRekey` ESP-key swap** (inbound/outbound reversed) surfaced once
  `installChildSA` became role-aware; corrected via `ChildSA.LocalIsInitiator`. Invisible in
  interop because macOS has no XFRM to install ESP keys into.
- **`applyIKERekeyResponse` role.** New rekeyed SA took `IsInitiator` from `oldSA.IsInitiator`; now
  that a responder-role SA can initiate a rekey, corrected to `true` (we sent Ni → we use SK_ei).
- **Interop witness must be strongSwan, not Ze.** Ze's plugin logs at INFO are not captured in the
  container; scenarios 07/09 first failed asserting a Ze log. Corrected to strongSwan-authoritative
  signals (the scenario-05 pattern): SA ESTABLISHED / "IKE_SA ze[N] rekeyed".

## Review-found bugs (multi-agent /ze-review, all fixed with regression tests)

- **BLOCKER — no responder half-open handshake timeout.** `runResponder`'s in-progress
  poll case just kept polling. A peer that sends IKE_SA_INIT, gets our response, then
  abandons (crash/restart/partition) before IKE_AUTH left the SA stuck: `responderBusy`
  pinned true and `tryResponderSAInit`'s CAS then dropped EVERY future IKE_SA_INIT from
  that peer (incl. its restarted reconnect) until daemon restart — a permanent per-peer
  wedge + a one-packet DoS. The initiator self-heals via `maxRetransmissions`; the
  responder had no equivalent. Fixed: `reapStaleHandshake` tears down when
  `time.Since(sa.CreatedAt) > responderHandshakeTimeout` (30s). **Lesson: a new responder
  role needs a half-open timeout mirroring the initiator's retransmit budget; "the peer
  will retransmit" only saves the live-peer case, not the abandon/restart case.** A second
  review pass then caught that the reaper must re-check `StateEstablished` first: the
  dispatch goroutine can complete the handshake between `runResponder`'s state switch and
  the reap, so a bare timeout check could orphan a just-established tunnel and leak its
  Child SA.
- **ISSUE — responder didn't select an ESP proposal.** `buildAuthResponse` echoed the
  full esp-group in SAr2 (RFC 7296 §2.7/§3.3 requires exactly one) and `createFirstChildSA`
  keyed from `Proposals[0]` regardless. Fine for single-proposal configs (interop 07);
  wrong for multi-proposal. Fixed: `selectResponderESP` negotiates one and narrows
  `sa.ESPGroup` (wiring the previously-dead `crypto.NegotiateESP`).
- **ISSUE — `ps.sa` unlocked-read race.** The responder made `ps.sa` mutable (dispatch
  `setSA`), but `TerminateAllSAs`/`TerminatePeerSA`/reconcile-stop still read it unlocked;
  `ps.Stop()` joins only the session goroutine, not dispatch. Fixed: route through `getSA()`.
- **ISSUE — `pendingIKESwap` key leak.** A peer re-initiating an IKE rekey before Deleting
  the old SA orphaned the first `newSA`'s `SKKeys`. Fixed: `setPendingIKESwap` clears the
  superseded keys.

## Reusable lessons

- When generalizing a hardcoded direction, grep for EVERY role-dependent site: send keys, receive
  keys, key-derivation input order (nonces AND SPIs), and downstream key-role assignment. A partial
  generalization compiles and passes same-role tests while silently corrupting the other role.
- In-process end-to-end tests (drive both peers' `LastSentMsg` into each other via `handleInbound`)
  are the deterministic correctness gate for a protocol role; interop against a real peer catches
  the things a self-consistent implementation cannot (here: ID-type constraints).

## Files

None recorded.

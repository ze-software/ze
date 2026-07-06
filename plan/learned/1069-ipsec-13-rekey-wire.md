# 1069 -- IKE/Child SA Rekey: Real CREATE_CHILD_SA Wire Exchange

## Context

ipsec-8 (learned 742) shipped Child/IKE SA rekey as a LOCAL key roll: on soft-lifetime
expiry the engine derived fresh keys and SPIs purely locally (`rekeyIKESA` even did
DH against its own public key -- "simulate DH with self") and never sent anything to
the peer. Because the peer never learned the new keys/SPIs, any configured lifetime
silently desynced a live tunnel at the first rekey. `spec-ipsec-13-rekey-wire` replaced
this with a real RFC 7296 §1.3.2/§1.3.3 CREATE_CHILD_SA exchange for Child SA and IKE SA
rekey, plus full §2.3 message-ID handling.

## Decisions

- **Route inbound to a single owner (Alternative A).** Post-establishment packets are
  routed off the shared `dispatchInbound` goroutine to the owning `PeerSession.inbound`
  channel (`routeInbound`, non-blocking send so one slow owner cannot wedge the shared
  loop); `maintainSA` becomes the sole owner of SA/childSA/message-ID state. Chosen over
  locking the lockless `SA` + threading `PeerSession` through `handleInbound`, because
  single-owner removes the whole race class and gives msg-ID/retransmit/collision one home.
- **Generalize SK framing, don't duplicate.** `buildEncryptedMessageEx(..., exchangeType,
  flags)` parameterizes the existing AEAD/CBC sealer; IKE_AUTH callers keep a wrapper.
- **Make-before-break.** Install the new Child SA before removing the old, then Delete the
  old via INFORMATIONAL (initiator) or keep it until the peer's Delete (responder).
- **Full §2.3 message-ID window (user-chosen scope).** Window-1 request/response counters,
  retransmit-cached-response, reset on IKE-SA rekey.
- **IKE-SA rekey is INITIATOR-only.** Responding to a peer-initiated IKE rekey makes Ze the
  new IKE SA's responder, which needs the SK encrypt/decrypt direction flipped (Ze hardcodes
  encrypt=SK_ei/decrypt=SK_er, correct only for the IKE-SA initiator). Deferred to
  spec-ipsec-14 (same root cause as the missing handshake responder); Ze logs+drops a
  peer-initiated IKE rekey rather than mis-encrypting.
- **Child rekey is non-PFS**, matching `createFirstChildSA` (which ignores the PFS config);
  keeps rekey consistent with the first child.

## Consequences

- Interop-verified against strongSwan 5.9.14: `test/ipsec-interop/scenarios/05-child-rekey`
  PASSES -- strongSwan parses Ze's REKEY_SA request, installs the new Child SA, and receives
  Ze's Delete for the old SA, stably every lifetime on one IKE SA.
- Both local-roll functions (`rekeyChildSA`, `rekeyIKESA`) removed; no layering.
- IKE-SA rekey re-keys the `SATable` and swaps the loop SA via an `ownedOutcome{newSA}`
  return; `runEstablished`/`maintainSA` now thread the `*SATable` (previously discarded).

## Gotchas (bugs found, several by the interop)

- **Stale post-handshake message ID.** The handshake set `NextMsgID = 1` for IKE_AUTH
  (`fsm.go`) but never advanced it at establishment; the first rekey reused ID 1 and
  strongSwan rejected it ("received message ID 1, expected 2, ignored"). Fixed: `NextMsgID++`
  when the SA reaches `StateEstablished` (correct for PSK and EAP, which pre-increment).
- **Hard-expiry tore down the session mid-rekey.** `newLifetimeState` sets soft = hard − jitter
  (jitter ≤ 10%), so a short lifetime initiates the rekey ~1-2s before hard-expiry, and
  `maintainSA` then returned `errTimeout` the same tick, killing the in-flight rekey (the
  tunnel churned a fresh handshake every lifetime). Fixed: gate both hard-expiry teardowns on
  `ps.pendingRekey == nil`.
- **Rekey install intolerant of no-XFRM kernels.** `createFirstChildSA` tolerates
  `ErrNotSupported`; the rekey install returned it, tearing down tunnels the first child was
  allowed to establish. Fixed: `installChildTolerant`.
- **ipsec-interop Docker harness was broken for EVERY scenario.** `.dockerignore` excluded
  `test/` with no exception, but `Dockerfile.ze` COPYs the cross-compiled binary from
  `test/ipsec-interop/ze-linux`; the build always failed. Fixed: `!test/ipsec-interop/ze-linux`
  in `.dockerignore`, and gitignored the 58MB artifact. The suite is a manual target
  (`make ze-ipsec-interop-test`), not in the default gate, so it had gone unnoticed as non-functional.
- **DPD interference.** DPD defaults on (interval 30, cannot be 0) and `sendDPD` is unencrypted;
  responses to Ze's own DPD/Delete requests now land in the owner loop and are dropped as
  out-of-window (we track pending state only for rekeys). Harmless to the rekey; DPD is a
  separate spec.
- Docker Desktop's Linux VM (macOS) has no XFRM/ESP; interop verifies the rekey at the control
  plane via strongSwan's logs (the dataplane assertions gate on XFRM availability).

## Files

- Created: `engine/msgid.go` (message-ID window + `pendingRekey`), `engine/rekey_wire_test.go`,
  `engine/established_test.go`, `test/ipsec-interop/scenarios/05-child-rekey/`.
- Modified: `engine/rekey.go` (wire exchange, removed local roll), `engine/inbound.go`
  (owner-loop handling), `engine/established.go` (owner routing, table threading, hard-expiry
  guard, retransmit), `engine/register.go` (`routeInbound`/`lookupPeerSession`),
  `engine/reconcile.go` (`inbound` channel + owner state), `engine/fsm.go` (NextMsgID fix),
  `engine/sa.go` (msg-ID cache fields), `engine/auth.go` (`buildEncryptedMessageEx`),
  `engine/child.go` (`installChildTolerant`), `.dockerignore`, `.gitignore`.
- Deferred: IKE-SA rekey responder + `06-rekey-responder` interop → spec-ipsec-14.

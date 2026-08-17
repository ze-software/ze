# Child SA and IKE SA rekey: the CREATE_CHILD_SA wire exchange

Rekey is a real exchange with the peer, not a local key roll. An earlier design
derived fresh keys and SPIs locally on soft-lifetime expiry and sent nothing.
The peer never learned the new keys, so any configured lifetime desynced a live
tunnel at the first rekey. Both local-roll functions were deleted when the wire
exchange landed.

## RFC obligations carried by this code

- RFC 7296 Section 1.3.2 and Section 1.3.3 define the CREATE_CHILD_SA exchange
  for Child SA rekey and IKE SA rekey.
- RFC 7296 Section 2.3 defines message-ID handling: a Window-1 request and
  response counter pair, a cached response for retransmission, and a reset of
  the counters when the IKE SA is rekeyed.

<!-- source: internal/component/ike/engine/msgid.go -- advanceMsgID, advanceExpectedMsgID, classifyInbound, cacheResponse -->
<!-- source: internal/component/ike/engine/rekey.go -- initiateChildRekey, applyChildRekeyResponse, initiateIKERekey -->

## Decisions

**One owner for post-establishment state.** Packets that arrive after
establishment are routed off the shared dispatch goroutine to the owning peer
session's inbound channel. The send is non-blocking, so one slow owner cannot
wedge the shared loop. `maintainSA` is then the sole owner of the SA, the child
SA and the message-ID state. The rejected alternative was to lock the lockless
`SA` and thread the peer session through `handleInbound`. A single owner removes
the whole race class and gives message-ID tracking, retransmission and collision
handling one home.

<!-- source: internal/component/ike/engine/register.go -- routeInbound, lookupPeerSession, dispatchInbound -->
<!-- source: internal/component/ike/engine/established.go -- runEstablished, maintainSA -->

**SK framing is generalized, not duplicated.** `buildEncryptedMessageEx` takes
the exchange type and the flags and parameterizes the existing AEAD and CBC
sealer. The IKE_AUTH callers keep a wrapper.

<!-- source: internal/component/ike/engine/auth.go -- buildEncryptedMessage, buildEncryptedMessageEx -->

**Make before break.** The new Child SA is installed before the old one is
removed. The initiator then deletes the old SA over an INFORMATIONAL exchange.
The responder keeps the old SA until the peer's Delete arrives.

**Child rekey is non-PFS.** This matches `createFirstChildSA`, which ignores the
PFS config, so a rekeyed child stays consistent with the first child.

<!-- source: internal/component/ike/engine/child.go -- createFirstChildSA -->

## Traps this code exists to avoid

**The message ID after the handshake.** The handshake sets the next message ID
to 1 for IKE_AUTH and does not advance it at establishment. The first rekey then
reuses ID 1, and a conforming peer rejects it. The counter is incremented when
the SA reaches `StateEstablished`, which is correct for PSK and for EAP, because
both pre-increment.

<!-- source: internal/component/ike/engine/fsm.go -- handleAuthResponse, handleEAPResponse -->

**Hard expiry against an in-flight rekey.** `newLifetimeState` sets the soft
lifetime to the hard lifetime minus `rekeyLead`. `rekeyLead` takes the larger of
two values. The first is `lifetimeJitter`, a random duration under 10% of the
lifetime. The second is the retransmit budget, `maxRetransmissions` multiplied by
`rekeyRetransmitTimeout` and capped at half the lifetime.

The jitter alone can return zero and put the trigger on the hard wall, so the
budget is the floor. A short lifetime therefore starts the rekey half a lifetime
before hard expiry, which is the gap that stops a rekey being killed on the tick
it starts.

The lead is the whole mechanism, because hard expiry itself is NOT gated on a
pending rekey. `maintainSA` tests `pendingRekey == nil` on the two soft-expiry
branches and on message-ID exhaustion, never on the two `hardExpired` branches.
RFC 7296 Section 2.8 is unconditional, so an in-flight rekey does not extend a
hard lifetime.

<!-- source: internal/component/ike/engine/rekey.go -- newLifetimeState, rekeyLead, lifetimeJitter -->
<!-- source: internal/component/ike/engine/fsm.go -- maxRetransmissions -->
<!-- source: internal/component/ike/engine/established.go -- maintainSA, rekeyRetransmitTimeout -->

**A kernel with no XFRM.** `createFirstChildSA` tolerates an unsupported
dataplane. The rekey install must tolerate it too, or it tears down tunnels that
the first child was allowed to establish. `installChildTolerant` is that path.

<!-- source: internal/component/ike/engine/child.go -- installChildTolerant, isXFRMUnsupported -->

**Dead peer detection interferes with the owner loop.** The probe is an
SK-protected INFORMATIONAL request with an empty inner chain, which RFC 7296
Section 1.4 requires. Responses to Ze's own probes and Delete
requests land in the owner loop and are dropped as out of window, because
pending state is tracked only for rekeys. This is harmless to the rekey.

<!-- source: internal/component/ike/engine/dpd.go -- dpdState -->

## Proof

`test/interop-ipsec/scenarios/05-child-rekey` runs against strongSwan 5.9.14.
strongSwan parses the REKEY_SA request, installs the new Child SA, and receives
the Delete for the old SA. The Docker VM on macOS has no XFRM or ESP, so the
scenario proves the control plane and the dataplane assertions gate on XFRM
being available.

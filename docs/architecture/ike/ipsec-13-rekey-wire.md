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

**The wire answer against the installed policy.** RFC 7296 Section 2.9 makes the
TS payloads of a rekey response a statement about the SA that was installed: "TS
payloads specify the selection criteria for packets that will be forwarded over
the newly set up SA." `newRekeyedChild` installs the set THIS exchange
negotiated, which both roles hand it as `sa.NegotiatedPairs`, so the scope on the
wire and the scope in the kernel are one value rather than two that agree by
accident. It also takes that set's orientation from the exchange role: the end
that sent Ni is TSi.

`narrowChildSelectors` keeps the two as one set on the responder. It answers the scope in use
while the peer's proposal covers it, and it refuses the exchange with
TS_UNACCEPTABLE when the narrowing does not cover that scope. Section 2.9.2
leaves no third answer: "The responder MUST NOT narrow down the Traffic Selectors
narrower than the scope currently in use", and Section 2.9 permits no answer
wider than the proposal. Before the refusal existed, a proposal that covered no
pair of the scope in use was answered with the intersection while the replacement
carried the retired pair's set. The peer then programmed one policy and Ze
programmed another, and traffic inside the difference was dropped at one end with
no notification.

<!-- source: internal/component/ike/engine/ts_narrow.go -- narrowChildSelectors, coversFloor -->
<!-- source: internal/component/ike/engine/rekey.go -- respondChildRekey, newRekeyedChild -->

**The answer the INITIATOR reads.** The same Section 2.9 sentence binds the other
role, and it binds it harder: the responder narrowed, so only the response says
what the peer will forward. `applyChildRekeyResponse` refuses a rekey response
that carries no TSi or TSr, then hands the payloads and the rekey floor to
`recordInitiatorSelectors`, which refuses an answer wider than what ze proposed
and an answer that does not cover the scope in use. What it records is what
`newRekeyedChild` installs.

RFC 7296 Section 2.21.3 settles what ze sends when it refuses: nothing. "Because
sending such error messages as an INFORMATIONAL exchange might lead to further
errors that could cause loops, such errors SHOULD NOT be sent." The pending rekey
is dropped, both selector sets reach the log, and the SA in use keeps carrying
traffic. Before this path read the payloads at all, ze installed the retired
pair's scope whatever the peer answered. Measured against strongSwan 5.9.14:
charon programmed `10.1.0.0/24 === 10.2.0.0/25` while ze kept `10.2.0.0/24 <->
10.1.0.0/24`.

<!-- source: internal/component/ike/engine/rekey.go -- applyChildRekeyResponse -->
<!-- source: internal/component/ike/engine/ts_narrow.go -- recordInitiatorSelectors, checkAnswerWithin -->

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

**Each end keeps its own request counter.** RFC 7296 Section 2.2 gives an
endpoint two current Message IDs: the next one it uses for a request it starts,
and the next one it expects from the peer. The two are independent, so the
original responder's first self-initiated request carries id 0 whatever ids the
handshake spent. `finishResponderEstablish` took the peer's IKE_AUTH id as this
side's next request id, which numbered every exchange this side started where
the peer expected none. `classifyInbound` matches the expected id exactly, so
the peer answered nothing: no liveness probe, no Delete and no rekey ever
completed from a responder-role Ze.

<!-- source: internal/component/ike/engine/responder.go -- finishResponderEstablish -->
<!-- source: internal/component/ike/engine/msgid.go -- advanceMsgID, advanceExpectedMsgID, classifyInbound -->

## Proof

`test/interop-ipsec/scenarios/child-rekey` runs against strongSwan 5.9.14
with Ze as the connection initiator. strongSwan parses the REKEY_SA request,
installs the new Child SA, and receives the Delete for the old SA.

`test/interop-ipsec/scenarios/responder-raises-child-rekey` runs the other
direction. strongSwan dials, so Ze is the original responder, and Ze's short ESP
lifetime makes Ze raise the CREATE_CHILD_SA. It is the only scenario where a
responder-role Ze speaks first, so it is what proves the two counters are
independent. With the old counter write restored, strongSwan parses no REKEY_SA
request at all.

Both scenarios gate their dataplane assertions on XFRM being available, because
a Docker host without XFRM or ESP still proves the control plane.

`test/ipsec/ipsec-child-rekey-xfrm.ci` reads the kernel directly, between two Ze
daemons on the real XFRM backend, with the exchange role crossed against the IKE
SA role. It compares the installed policy selectors across the whole
make-before-break window and fails when a rekey moves one.

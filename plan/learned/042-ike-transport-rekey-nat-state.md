# Learned: a Child SA's mode belongs to the Child SA, and a red proof must break what it claims

`spec-ike-transport-rekey-nat-state` fixed the IKEv2 engine's handling of a
transport-mode Child SA rekeyed behind a NAT after its IKE SA was rekeyed. The
mode each exchange installs is now computed once per exchange and passed to the
two role entry points (`narrowChildSelectorsIn`, `recordInitiatorSelectorsIn`
in `internal/component/ike/engine/ts_narrow.go`), the installed mode lives only
in `ChildSA.Mode`, `PeerRequestedTransport` and `transportSelectorsInPlay` are
gone, and `SA.UseTransportMode` is the IKE_AUTH carrier only. A rekey request
without USE_TRANSPORT_MODE now installs tunnel (RFC 7296 Section 1.3.1). The
reference pages are `docs/architecture/ike/ipsec-7-ikev2-engine.md`,
`ipsec-8-ikev2-child-xfrm.md` and `rfcgate-1b-rfc7296-pilot.md`.

## What outlives the spec

1. **A negotiation outcome stored on a longer-lived object is lost when that
   object is replaced.** The transport flags were the result of one creation
   exchange, kept on the IKE SA and read by every later exchange as if they
   described the installed Child SA. An IKE SA rekey builds the successor
   without them (RFC 7296 Section 2.8: the new IKE SA inherits the Child SAs,
   not the flags), so the next Child SA rekey judged a transport SA as tunnel.
   Copying the flags into the successor would have added a third writer. The
   cure was to make the outcome a value of the exchange and keep the lasting
   fact on the object that owns it. The problem class is
   `plan/journal/one-state-held-in-two-fields.md`.
2. **A red proof must break the obligation its claim names, not a neighbour.**
   Two proofs here first went red for the wrong reason. The single-address gate
   on the rekey path never decides a transport rekey: the RFC 7296 Section 2.9.2
   floor answers first, so with the gate replaced by `false` the unit stayed
   green, and the negative RFC7296-2.23.1-1 tag was removed from it. The
   RFC7296-1.3.1-2 positive record was ruled not genuine by the test judge and
   re-taken with a break of the deletion itself (39cd72b1f1). The rule is
   `docs/contributing/rfc-conformance-gates.md` ("one violation the claim
   names", judged by `ze-test-judge`).
3. **An interop peer can make a scenario unable to tell the fix from the
   defect.** strongSwan answers Ze's initiator Child SA rekey in Ze's own
   addresses, so the initiator scenario passes with the substitution broken. It
   stays as proof that the path works, and the discriminating proof is the unit
   tests that send a peer answer in the peer's address space
   (`docs/architecture/testing/interop.md`, the after-IKE-rekey paragraph).

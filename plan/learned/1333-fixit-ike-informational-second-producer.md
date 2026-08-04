# 1333 -- A second INFORMATIONAL handler tore tunnels down with no key material

## Context

`handleInformational` (`internal/component/ike/engine/inbound.go`) was a second handler for
post-establishment INFORMATIONAL messages, reached through `handleInbound`'s
`case StateEstablished` whenever `routeInbound` found the owner loop did not hold the SA. It
took no transport, so it answered nothing, and it never decrypted: it walked `msg.Payloads`,
which is the OUTER chain, and `wire.Message.ReadFrom` does not decrypt an SK payload. A
genuine encrypted INFORMATIONAL therefore showed it one entry, a `*wire.PayloadSK`, and it
never saw a Delete. The reverse case was the defect. A CLEARTEXT datagram naming the right
SPI pair and carrying a plaintext IKE Delete reached `sa.State = StateDead` with no key
material consulted, so an off-path attacker could tear an established tunnel down. RFC 7296
Section 2.4 forbids concluding the peer has failed from an unauthenticated message.

## Decisions

- Deleted the second handler over teaching it to answer. An answer writes `cacheResponse` and
  reads `SKKeys`, both owned by `maintainSA` alone, so answering on the shared dispatch
  goroutine is the race the single-owner model exists to remove.
- Dropped the packet and let the peer retransmit, over blocking or queueing it. RFC 7296
  Section 2.1 makes the initiator retransmit, and `routeInbound`'s queue-full arm already
  relied on that.
- Removed the initiator SA by PEER NAME, over a removal by SPI pair or a pruner over the whole
  table. `runResponder` already removed its own SA the same way, so both roles share one shape.
- Kept the queue-full drop as it was, over a blocking send that would stall every peer on the
  shared dispatch goroutine.

## Consequences

- A handler that walks `msg.Payloads` on a post-establishment message sees the OUTER chain.
  It can only ever see payloads an attacker sent in the clear, because a real peer's sit
  inside the SK payload. Any such handler is a cleartext-only handler whatever it was meant
  to be, and that shape is the thing to look for, not the missing decrypt call.
- One handler now serves every post-establishment message, and it decrypts first. A second
  producer for a message class is the thing to refuse, not the thing to harden.
- A legitimate peer request that lands in the establish window is dropped and answered on the
  retransmit, roughly 500ms later. The answer is late, never lost.
- The SATable removal also corrects `ze_ipsec_sa_count` and `ze_ipsec_tunnel_up`, which had
  been counting every leaked zombie as a live tunnel.

## Gotchas

- **Go evaluates the arguments of a deferred CALL where the defer is written.** A deferred
  `table.Remove(sa.InitiatorSPI, sa.ResponderSPI)` therefore captures the responder SPI as
  the zero `newInitiatorSA` left, and then deletes a key `handleSAInitResponse` already
  replaced through `UpdateKey`. Reading the fields at return time does not save it either,
  because an IKE rekey swaps in a different SA under a new pair. Removing by peer name is
  what makes the deferred form correct.
- The two defers are ordered deliberately, and the order is invertible by accident.
  `defer sa.forgetKeys()` is registered BEFORE `defer table.RemoveByPeer(...)` so that it
  RUNS AFTER it: defers unwind last-first, and an SA whose keys are zeroed while it is still
  in the table would let a packet still being dispatched decrypt against a zeroed key.
- The routing test that proves the removal, `TestRteInitiatorCycleLeavesNoTableEntry`, closes
  `stopCh` before the cycle starts, so it never reaches `UpdateKey` and would pass against a
  by-SPI-pair removal too. The discriminating proof lives in a different file:
  `TestIcyEstablishedCycleLeavesNoTableEntry` (`initiator_cycle_test.go`) drives a full
  established cycle and an IKE-rekeyed one. An independent reviewer reading only the spec's
  named test file reported the coverage as missing, so name the sibling file in the spec when
  the discriminating test does not live where the spec says.

## Files

- `internal/component/ike/engine/inbound.go`
- `internal/component/ike/engine/fsm.go`
- `internal/component/ike/engine/register.go`
- `internal/component/ike/engine/rfc7296_routing_test.go`
- `internal/component/ike/engine/initiator_cycle_test.go`

# Learned: Child SA ports follow IKE, a TLS 1.2 server chooses its session_id, and a rekey restates its mode

`spec-ike-eap-rfc-defects` closed two RFC defects. A peer behind a NAT that
rewrote its IKE source port received ESP on 4500 (RFC 3948 Section 2.1), and
an EAP-TLS 1.2 full handshake carried no session_id (RFC 5216 Section 2.1.1).

## What changed

- Every Child SA takes its UDP encapsulation ports from the IKE SA: the
  authenticated peer endpoint for the remote port, the bound NAT-T socket for
  the local one (`childEncapPorts`, `localEncapPort`,
  `internal/component/ike/engine/create_child.go`). With neither, the Child SA
  is refused; no port is guessed. A rekey takes them from the IKE SA, never from
  the retired child (`rekey.go`). The EAP responder adopts the endpoint before
  it builds the Child SA (`handleResponderEAP`).
- A non-MOBIKE peer that moves after establishment: tunnel Child SAs migrate in
  place, transport ones owe a rekey onto the new ports (`rebind.go`, raised as a
  level trigger by `serviceChildRekeyTrigger`).
- A transport-mode Child SA rekey carries USE_TRANSPORT_MODE, the responder
  echoes it, and an answer without it is refused with a rekey hold
  (`initiateChildRekey`, `inbound.go`).
- `crypto/tls` is patched at build time (`internal/le/go/stdpatch`,
  `docs/contributing/patching-the-standard-library.md`): a TLS 1.2 full
  handshake with no ticket sends a fresh 32-octet session_id. TLS 1.2 tickets
  are suppressed in the authenticator. Every patch carries a review stamp.
- Interop: a nat.conf port column, a rebind step that swaps the NAT's port rule
  and flushes conntrack, and four scenarios, all green against strongSwan 5.9.14
  with their reds recorded.

## What outlives the spec

1. **A rekey restates the mode it rekeys.** RFC 7296 Section 1.3.3 expects the
   rekey to carry the notifications of the original exchange. Without
   USE_TRANSPORT_MODE strongSwan read the rekey as tunnel mode, skipped its own
   NAT selector substitution and answered TS_UNACCEPTABLE. The defect looked
   like a selector-address problem. Reading strongSwan's `child_create.c` found
   the real cause before a wrong fix landed.
2. **Every refusal branch of a level-triggered retry needs a hold.** The soft
   lifetime and an owed rebind rekey fire on every tick. A refusal that deletes
   the new pair without holding the next attempt turns into a rekey and Delete
   every second.
3. **When no peer can reach the success path, assert the refusal.** No IKEv2
   EAP-TLS initiator sends the RFC 7627 extended master secret on TLS 1.2, so
   Ze refuses the RFC 5216 MSK export. `responder-eap-tls12` asserts the
   handshake reaches Finished, the refusal line, and no SA (owner decision
   2026-10-21). Its red is a crypto/tls patched to export without EMS, under
   which charon authenticates.
4. **An interop scenario can be green and unable to go red.** The D1b mutant
   stayed green because earlier EAP rounds already adopt the endpoint. The unit
   test carries that proof. Record "no reachable red" rather than claim one.
5. **Lab traps.** A `git archive` export needs the ignored
   `gokrazy/docker-host/builddir/.../go.sum`, and its path must be short enough
   for the forwarded Docker socket. A lab-wide `ZE_LOG_LEVEL` does not raise
   the ike subsystem: a scenario that reads an INFO line sets `ZE_LOG_IKE`.

<!-- ste: ignore-file preserved verbatim from a design pass; it quotes RFC 7296 at length. -->

# Design: `RFC7296-2.3-6`, INVALID_MESSAGE_ID with the exposure bounded

Owner ruling (recorded in `plan/handover/02-rfcgate-1b-pilot-wp1-wp2-wp6-wp12.md`):
**implement it, with the exposure bounded.** This design says exactly what bounds it.

## 1. The obligation, verbatim

> "The INVALID_MESSAGE_ID notification is sent when an IKE Message ID outside the supported
> window is received. This Notify message MUST NOT be sent in a response; the invalid
> request MUST NOT be acknowledged. Instead, inform the other side by initiating an
> INFORMATIONAL exchange with Notification Data containing the four-octet invalid Message
> ID. Sending this notification is OPTIONAL, and notifications of this type MUST be rate
> limited."

`rfc/full/rfc7296.txt:1503-1509`.

**Read the MUST carefully. The obligation of this row is the RATE LIMIT, not the sending.**
Sending is stated OPTIONAL in the same sentence. Every bound below is therefore fully
conformant, and none of them is a scope reduction.

The neighbouring row `RFC7296-2.3-5` ("MUST NOT be sent in a response; the invalid request
MUST NOT be acknowledged") is already landed and proven by
`TestOsrOutOfWindowRequestIsNotAcknowledged`. Nothing in this design may redden it.

## 2. What Ze does today

| Fact | Producing function | `file:line` |
|------|--------------------|-------------|
| An established-SA message is classified BEFORE it is decrypted | `handleOwnedInbound` | `internal/component/ike/engine/inbound.go:46`, decrypt at `:75` |
| An out-of-window message is dropped with no datagram | `handleOwnedInbound`, `inboundInvalid` arm | `internal/component/ike/engine/inbound.go:68-71` |
| Exactly ONE out-of-window path decrypts today | same arm, INFORMATIONAL **response** only | `internal/component/ike/engine/inbound.go:59-67` |
| `NotifyInvalidMessageID` is declared and has no non-test referent | -- | `internal/component/ike/wire/payload_notify.go:36` |
| A token-bucket limiter exists, inbound only | `newInboundRateLimiter` | `internal/component/ike/engine/register.go:416-445` |
| Originating an INFORMATIONAL request takes the one request window | `sendDPD` | `internal/component/ike/engine/dpd.go:132` |

The answer to the handover's research question: **no path decrypts an out-of-window
REQUEST.** The single decrypting path at `:59-67` is guarded by
`isResponse && ExchangeType == Informational`.

## 3. The exposure, and the three bounds that remove it

The naive emission is an attacker-triggered self-denial-of-service. `classifyInbound` runs
before decryption, so an off-path attacker who reads the cleartext SPI pair sends one
datagram at an out-of-window Message ID. Ze then originates an INFORMATIONAL, which takes
`reserveRequestWindow`. The window is 1, so one forged datagram stalls that SA's own DPD,
Delete and rekey for the whole 30 second `requestWindowTimeout`.

| Bound | What it stops | Why it is conformant |
|-------|---------------|----------------------|
| **B1. Emit only after the message AUTHENTICATES.** Decrypt the out-of-window REQUEST first. Emit only when `decryptAndParse` returns no error | An off-path attacker cannot forge bytes that decrypt under SK_ei, so it cannot trigger any emission at all | Sending is OPTIONAL. Nothing requires Ze to answer a message it cannot authenticate |
| **B2. Emit only when the request window is FREE.** Call `reserveRequestWindow` and skip the notify when it returns false | A courtesy notification never displaces Ze's own DPD probe, Delete or rekey. This also removes the replay-driven stall, which B1 alone does not | Sending is OPTIONAL |
| **B3. Rate limit, per SA.** A token bucket on the SA, refilled by the maintainSA clock | A peer replaying captured ciphertext at many old Message IDs draws at most the bucket's rate | This IS the row's MUST |

B1 does not make B2 redundant. A replay attacker holding old ciphertext CAN produce a
message that decrypts, because RFC 7296 §2.2 makes the Message ID the replay protection and
a replayed old id is exactly what lands out of window. B1 removes the off-path attacker. B2
removes the self-denial for every attacker, including the replaying one. B3 caps the cost.

## 4. Implementation

**File: `internal/component/ike/engine/inbound.go`, `handleOwnedInbound`, `inboundInvalid` arm.**

After the existing INFORMATIONAL-response branch and before the debug log at `:68`, add a
REQUEST branch:

1. Guard on `!isResponse`. A response must never draw this (`RFC7296-2.3-5`, and
   `RFC7296-3.1-12`).
2. `decryptAndParse(sa, &msg, pkt.Data)`. On error, fall through to the existing drop. Do
   not log at a level that gives an off-path attacker a signal.
3. On success, call a new `ps.sendInvalidMessageID(sa, msg.Header.MessageID, tr, log)`.
4. Return `ownedOutcome{}`. **Do NOT set `peerAlive`.** An out-of-window message is not
   evidence for the RFC 7296 §2.4 liveness path, and crediting it there would let a replay
   mask a dead peer. This is the same reasoning the existing DPD correlation uses.

**File: `internal/component/ike/engine/msgid.go` (or a new `notify_invalid_msgid.go`).**

`sendInvalidMessageID(sa, badID, tr, log)`:

1. `if !sa.invalidMsgIDLimiter.allow() { return }` -- B3, the row's MUST.
2. `if !sa.reserveRequestWindow() { return }` -- B2. Log at debug and return.
3. Build the four-octet Notification Data: `binary.BigEndian.PutUint32(data[:], badID)`.
   The RFC fixes the length at four octets, so build it into a `[4]byte`, never a slice
   whose length a caller can vary.
4. `buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: &wire.PayloadNotify{
   NotifyMsgType: wire.NotifyInvalidMessageID, NotificationData: data[:]}}},
   sa.NextMsgID, wire.ExchangeInformational, initiatorFlag(sa))`.
   On a build error call `sa.releaseRequestWindow()` and return, exactly as `sendDPD` does
   at `dpd.go:145-148`.
5. `sendRaw`, then `sa.advanceMsgID()`.

Check `wire.PayloadNotify`'s ProtocolID and SPI fields before writing step 4. For a notify
about the IKE SA itself the Protocol ID is 0 and the SPI is empty. Read
`internal/component/ike/wire/payload_notify.go` rather than assuming.

**The limiter.** Add a small per-SA token bucket. Do NOT reuse `inboundRateLimiter`: that
one carries a mutex it does not need here, because the SA's state is owned solely by the
maintainSA goroutine (`ai/rules/design-principles.md`, A-5 in the spec). One token per
second with a burst of three is enough: the notification is a courtesy to a peer that is
already confused, and a conforming peer draws at most one.

## 5. Tagged tests

New file `internal/component/ike/engine/rfc7296_invalidmsgid_test.go`.

| Polarity | Test | Asserts | Mutation that MUST redden it |
|----------|------|---------|------------------------------|
| positive | `TestImiRateLimitCapsTheNotification` | drive many authenticated out-of-window requests through `handleOwnedInbound` and count the INFORMATIONAL datagrams that reach the peer transport. The count is capped by the bucket and is strictly less than the number of requests | delete the `allow()` guard: the count rises to one per request |
| negative | `TestImiUnauthenticatedRequestDrawsNothing` | a datagram at an out-of-window Message ID whose ciphertext does NOT decrypt draws no datagram at all. This is the bound that makes the emission non-attacker-triggerable | move the emission before the decrypt: the forged datagram draws a notify |
| positive | `TestImiNotificationCarriesTheFourOctetMessageID` | the emitted message is a REQUEST (no Response flag), exchange INFORMATIONAL, carrying one Notify of type 9 whose data is exactly four octets and equals the invalid id big-endian | write the id little-endian, or send three octets: the assertion names the octets |
| negative | `TestImiHeldWindowSuppressesTheNotification` | with the request window already held by a Delete, an authenticated out-of-window request draws NOTHING, and the Delete's window is still held afterwards | delete the `reserveRequestWindow` guard: a second request goes out beside the Delete |

**Anti-vacuity.** Every negative above asserts an ABSENCE, which is the trap this spec has
recorded. Each one therefore needs its control in the same test: the SAME fixture with the
one bounding condition removed DOES draw the notification. Without the control the test
passes with the whole feature deleted.

`TestOsrOutOfWindowRequestIsNotAcknowledged` must stay green. It asserts that an
out-of-window request draws no RESPONSE. It builds its bad requests with the peer SA's real
keys, so they WILL now decrypt and WILL draw an INFORMATIONAL request. Read that test
before you start: its `rtxExpectSilence` calls will need to distinguish "no response" from
"no datagram", and that is a real change to an existing tagged test. It is a STRENGTHENING
under the owner's standing approval, but it must be mutation-verified again afterwards.

## 6. Id allocation

Section 2.3 high-water mark from committed HEAD is **8** (ids 2, 4, 5, 7, 8). Appendix A
states the row must take `2.3-9` or higher, and that ordinal 6 can never be allocated. This
row therefore lands as **`RFC7296-2.3-9`**.

Re-verify before writing:

    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-2\.3-[0-9]*' | sort -V | tail -1

Row text, one physical line:

    - [ ] [RFC7296-2.3-9] [MUST] Sending this notification is OPTIONAL, and notifications of this type MUST be rate limited (§2.3)

Correct the Appendix A row at `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md:1071` to `2.3-9` in
the same change, and drop its "held for an owner ruling" note.

## 7. What must not break

| Invariant | Guard |
|-----------|-------|
| `RFC7296-2.3-5`, no response and no acknowledgement | the emission is a REQUEST carrying its own new Message ID. `TestOsrOutOfWindowRequestIsNotAcknowledged` and the third test above both assert the Response flag is clear |
| `RFC7296-3.1-12`, never answer a message marked as a response | the `!isResponse` guard is the first condition |
| `RFC7296-2.3-8`, one outstanding request | B2 is that guard, and `TestOsrRetireOnlyFreesItsOwnWindow` covers the window bookkeeping |
| `RFC7296-2.2-2`, the Message ID ceiling | the emission calls `advanceMsgID`, which freezes at the ceiling |
| The DPD liveness path | the new branch returns without `peerAlive`, so a replay cannot mask a dead peer |

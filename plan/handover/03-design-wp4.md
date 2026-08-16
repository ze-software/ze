<!-- ste: ignore-file preserved verbatim from a design pass; it quotes RFC 7296 at length. -->

# WP-4 design -- COOKIE and INVALID_KE_PAYLOAD retry

Rows: `RFC7296-1.2-5`, `RFC7296-1.2-6`, `RFC7296-2.6-3`, `RFC7296-2.6-4`, `RFC7296-2.6-5`,
`RFC7296-2.6.1-1`.
Source spec: the rfcgate-1b RFC 7296 pilot spec, phase list item 9.

**Read-only design. No tracked file was modified.** Every `file:line` below was read in the
working tree on 2026-07-31. Other agents are editing `internal/component/ike/engine/`
(`msgid.go`, `responder.go` and three `rfc7296_*_test.go` files carry a 12:2x mtime), so
engine line numbers WILL move. Every citation names the FUNCTION as well as the line.
**Re-locate by function name, and re-read before quoting a line into a tag.**

**A naming collision the implementer will meet.** The 2026-07-30 re-triage renumbered the
work packages (the rfcgate-1b RFC 7296 pilot spec). "WP-4" now names "Notify
vocabulary and the error-response path" (12 rows), and this package's work sits in the new
"WP-7, COOKIE, DH-group retry, KE payload agreement" (10 rows). The brief and the phase list
use the OLD numbering, and this document follows the brief. The new WP-7 carries 10 rows
where this design covers 6; the extra four are almost certainly `1.3-2`, `3.4-1`, `3.4-2` and
`3.4-3`, and section 10 says what to do about that. **Say which numbering a commit message
uses.**

---

## 0. Verdict

| Row | Appendix A class | This design's verdict | Needs production code? |
|-----|------------------|-----------------------|------------------------|
| `RFC7296-1.2-5` | **NOT IMPL** | **absent.** No `NotifyInvalidKEPayload` case exists on the initiator; the response kills the SA | yes |
| `RFC7296-1.2-6` | **NOT IMPL** | **absent, and satisfied for free once `1.2-5` lands** if the retry reuses `buildSAInitRequest` | yes (as a consequence) |
| `RFC7296-2.6-3` | **NOT IMPL** | **absent.** No COOKIE producer, and `PayloadNotify` enforces no data length | yes |
| `RFC7296-2.6-4` | **NOT IMPL** | **absent.** No `NotifyCookie` case on the initiator | yes |
| `RFC7296-2.6-5` | **NOT IMPL** | **absent.** No cookie is ever expected, so nothing compares one | yes |
| `RFC7296-2.6.1-1` | **NOT IMPL** | **absent.** The retry loop the MUST NOT governs does not exist | yes (as a consequence) |

**All six are absent. WP-4 is the first package in this spec that is mostly production code
rather than mostly tests.** No row can be discharged by absence, and section 1.7 says why
that reading is not available here even though it was available for WP-5.

**Two defects fall out of the reading, and neither is in Appendix A.**

- **D-1 (interop, certain).** Ze can NEVER establish with a peer that prefers a different
  Diffie-Hellman group. `handleSAInitResponse` marks the SA dead, `runInitiator`'s loop does
  not test for dead, so Ze burns its whole retransmit budget re-sending the SAME
  wrong-group request, times out, and reconnects with the same group forever. Section 1.1.
- **D-2 (availability, certain).** A single spoofed UDP datagram carrying the configured
  peer's source address wedges that peer's only half-open slot for 30 seconds. Section 3.2.
  COOKIE is the RFC's answer to exactly this, and it is the reason the responder half of
  this package is worth building rather than only the initiator half.

---

## 1. The rows

### 1.1 `RFC7296-1.2-5` -- the initiator MUST retry with the corrected DH group

#### The obligation, verbatim

> "Because the initiator sends its Diffie-Hellman value in the IKE_SA_INIT, it must guess
> the Diffie-Hellman group that the responder will select from its list of supported
> groups.  If the initiator guesses wrong, the responder will respond with a Notify payload
> of type INVALID_KE_PAYLOAD indicating the selected group.  In this case, the initiator
> MUST retry the IKE_SA_INIT with the corrected Diffie-Hellman group."

`rfc/full/rfc7296.txt:662-668`. The MUST is on `:667-668`.

Appendix A (the rfcgate-1b RFC 7296 pilot spec) quotes from "If the initiator
guesses wrong" onward and drops the two setup sentences. The elision does not widen or
narrow the obligation, so it is legitimate. Keep the row text as Appendix A has it.

The same rule is restated normatively for the KE payload itself:

> "If the selected proposal uses a different Diffie-Hellman group (other than NONE), the
> message MUST be rejected with a Notify payload of type INVALID_KE_PAYLOAD.  See also
> Sections 1.2 and 2.7."

`rfc/full/rfc7296.txt:4991-4993`. That sentence is the RESPONDER's obligation and is
`RFC7296-1.3-2`, already `impl-untested` in Appendix A and already gated (section 9).
`1.2-5` is the INITIATOR's half and is the one WP-4 owns.

And §2.21.1 confirms the initiator is expected to act on it rather than give up:

> "In an IKE_SA_INIT exchange, any error notification causes the exchange to fail.  Note
> that some error notifications such as COOKIE, INVALID_KE_PAYLOAD or
> INVALID_MAJOR_VERSION may lead to a subsequent successful exchange.  Because all error
> notifications are completely unauthenticated, the recipient should continue trying for
> some time before giving up.  The recipient should not immediately act based on the error
> notification unless corrective actions are defined in this specification, such as for
> COOKIE, INVALID_KE_PAYLOAD, and INVALID_MAJOR_VERSION."

`rfc/full/rfc7296.txt:3263-3271`. This paragraph is load-bearing for the whole package: it
is the RFC's own statement that acting on an unauthenticated COOKIE or INVALID_KE_PAYLOAD is
correct, and that the acting must be BOUNDED ("continue trying for some time before giving
up"). Section 4 turns that into the specific bounds.

#### What Ze does today

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| The initiator's notify switch handles four types, and `NotifyInvalidKEPayload` is not one of them | `handleSAInitResponse` | `internal/component/ike/engine/fsm.go` (`NoProposalChosen` `:415`, `SignatureHashAlgorithms` `:420`, `NATDetectionSourceIP` `:426`, `NATDetectionDestIP` `:437`) |
| An INVALID_KE_PAYLOAD response carries no SA, KE or Nonce, so it falls to the completeness gate and the SA dies | `handleSAInitResponse` | `internal/component/ike/engine/fsm.go` |
| `runInitiator`'s wait loop tests only for `StateEstablished`, never for `StateDead` | `runInitiator` | `internal/component/ike/engine/fsm.go` |
| A dead SA therefore re-sends the SAME wrong-group request until the retransmit budget is spent | `runInitiator` | `internal/component/ike/engine/fsm.go` (`sa.LastSentMsg` re-sent at `:154`) |
| The next cycle rebuilds the DH from the same config index, so it repeats forever | `newInitiatorSA` | `internal/component/ike/engine/initiator.go` (`ikeGroup.Proposals[0].DHGroup`) |
| Ze as RESPONDER does send the notify | `handleSAInitRequest` | `internal/component/ike/engine/responder.go`, sending at `:156` |

**Verdict: absent.** This is defect D-1. Ze is a conforming SENDER of INVALID_KE_PAYLOAD and
a non-conforming RECEIVER of it, and the receiving gap is unrecoverable rather than merely
slow.

**One correction to the spec's own record.** the rfcgate-1b pilot spec at its line 1797 (the spec is closed; its record is the rfcgate-1b RFC 7296 pilot spec)
says of the CREATE_CHILD_SA half that `respondIKERekey` "builds no Notify". That is now
stale: `respondIKERekey` (`internal/component/ike/engine/rekey.go`) builds an
INVALID_KE_PAYLOAD at `:548`, and two tagged tests cover it
(`TestNegRekeyRejectsMismatchedKEGroup` and `TestNegSharedSecretRefusesWrongLength`,
`internal/component/ike/engine/rfc7296_negotiation_test.go` and `:336`). Fix the spec
line in the same commit. It does not change WP-4's scope: §1.3's rekey retry is phrased
"the initiator will probably retry" (`rfc/full/rfc7296.txt:752-755`), which is not a MUST.

### 1.2 `RFC7296-1.2-6` -- the retry MUST re-propose the full suite set

#### The obligation, verbatim

> "The initiator MUST again propose its full set of acceptable cryptographic suites because
> the rejection message was unauthenticated and otherwise an active attacker could trick
> the endpoints into negotiating a weaker suite than a stronger one that they both prefer."

`rfc/full/rfc7296.txt:668-681` (the sentence is split by a page break at `:670-678`; the
MUST is on `:668`, and the rationale resumes at `:679`).

Appendix A truncates at "was not authenticated". **Two problems with that.** The
source says "was unauthenticated", not "was not authenticated", and the dropped clause is
the entire security rationale. Restore the full sentence; section 11 gives the row text.

#### What Ze does today

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| The one request builder always encodes the WHOLE configured proposal set | `buildSAInitRequest` | `internal/component/ike/engine/initiator.go` calling `buildWireIKEProposals` |
| `buildWireIKEProposals` iterates every configured proposal | `buildWireIKEProposals` | `internal/component/ike/engine/initiator.go` |
| There is no retry, so there is no message for the obligation to govern | `runInitiator` | `internal/component/ike/engine/fsm.go` is the only `buildSAInitRequest` call outside tests |

**Verdict: absent, but cheap.** The obligation is vacuous today because no retry exists.
The moment the retry reuses `buildSAInitRequest` rather than narrowing the offer to the
responder's named group, `1.2-6` holds by construction. **The design choice that makes it
hold is stated in section 2.4 and the mutation that reddens it is in section 6.2.** This is
the single most attacker-relevant row in the package: an off-path attacker who can forge one
INVALID_KE_PAYLOAD gets to choose Ze's DH group, and if Ze also narrowed its cipher offer in
response it would get to choose the cipher too.

### 1.3 `RFC7296-2.6-3` -- COOKIE data MUST be 1..64 octets

#### The obligation, verbatim

> "When a responder detects a large number of half-open IKE SAs, it SHOULD reply to
> IKE_SA_INIT requests with a response containing the COOKIE notification.  The data
> associated with this notification MUST be between 1 and 64 octets in length (inclusive),
> and its generation is described later in this section."

`rfc/full/rfc7296.txt:1799-1803`. The MUST is on `:1801-1802`. Appendix A quotes
only the MUST sentence, which is correct: the SHOULD in the preceding sentence is a separate
obligation nobody extracted, and section 10 raises that.

#### What Ze does today

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| `NotifyCookie` is declared and has NO non-test referent anywhere in the tree | the constant only | `internal/component/ike/wire/payload_notify.go` (`grep -rn 'NotifyCookie' internal/ --include=*.go` returns that one line) |
| `PayloadNotify` enforces NO length rule on `NotificationData` at encode | `PayloadNotify.WriteTo` | `internal/component/ike/wire/payload_notify.go` (`copy(buf[off+n:], p.NotificationData)` at `:104`) |
| Nor at parse | `PayloadNotify.ReadFrom` | `internal/component/ike/wire/payload_notify.go` |
| The only length bound anywhere near this path is the 512-octet notify buffer | `sendSAInitNotify` | `internal/component/ike/engine/responder.go` |

**Verdict: absent.** Note the asymmetry the row must cover: `2.6-3` binds Ze as a cookie
GENERATOR, and `2.6-4` makes Ze ECHO a peer's cookie, so the 1..64 bound is needed on both
the mint path and the echo path. A peer that sends a 600-octet COOKIE must not have Ze echo
it back. The existing `sendSAInitNotify` guard would drop such a message rather than
truncate it (`responder.go`), which fails safe but reports the wrong reason; the
echo path needs its own explicit `2.6-3` check so the log names the real fault.

### 1.4 `RFC7296-2.6-4` -- the initiator MUST retry with COOKIE first and everything else unchanged

#### The obligation, verbatim

> "If the IKE_SA_INIT response includes the COOKIE notification, the initiator MUST then
> retry the IKE_SA_INIT request, and include the COOKIE notification containing the
> received data as the first payload, and all other payloads unchanged."

`rfc/full/rfc7296.txt:1803-1807`. Appendix A quotes this exactly. No change needed.

The exchange diagram that fixes the shape:

>    Initiator                         Responder
>    -------------------------------------------------------------------
>    HDR(A,0), SAi1, KEi, Ni  -->
>                                 <--  HDR(A,0), N(COOKIE)
>    HDR(A,0), N(COOKIE), SAi1,
>        KEi, Ni  -->
>                                 <--  HDR(A,B), SAr1, KEr,
>                                          Nr, [CERTREQ]

`rfc/full/rfc7296.txt:1809-1816`. And the state rule that follows it:

> "The first two messages do not affect any initiator or responder state except for
> communicating the cookie.  In particular, the message sequence numbers in the first four
> messages will all be zero and the message sequence numbers in the last two messages will
> be one."

`rfc/full/rfc7296.txt:1823-1826`.

This is three separate testable properties, and the design must hit all three:

1. The COOKIE notify is the FIRST payload of the retry.
2. `SAi1`, `KEi` and `Ni` are byte-identical to the first attempt. Not "equivalent" --
   **the same nonce and the same DH public value**. A fresh nonce would be a new exchange.
3. Message ID stays 0, and the initiator SPI `A` stays the same.

#### What Ze does today

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| The initiator's notify switch has no `NotifyCookie` case | `handleSAInitResponse` | `internal/component/ike/engine/fsm.go` |
| A COOKIE-only response has no SA, KE or Nonce, so it dies at the completeness gate | `handleSAInitResponse` | `internal/component/ike/engine/fsm.go` |
| The request builder has no cookie parameter and no way to prepend a payload | `buildSAInitRequest` | `internal/component/ike/engine/initiator.go`, payload slice built at `:56-64` |
| Payload order on the wire IS the slice order, so "first payload" is expressible | `Message.WriteTo` | `internal/component/ike/wire/message.go` writes `m.Payloads[i]` in order |

**Verdict: absent.** Property 2 is the one an implementation gets wrong: the obvious
implementation calls `newInitiatorSA` again, which mints a fresh nonce
(`internal/component/ike/engine/initiator.go`) and a fresh DH key. Section
6.2 gives the mutation that catches it.

### 1.5 `RFC7296-2.6-5` -- a mismatched cookie MUST be ignored, not rejected

#### The obligation, verbatim

> "When one party receives an IKE_SA_INIT request containing a cookie whose contents do not
> match the value expected, that party MUST ignore the cookie and process the message as if
> no cookie had been included; usually this means sending a response containing a new
> cookie.  The initiator should limit the number of cookie exchanges it tries before giving
> up, possibly using exponential back-off.  An attacker can forge multiple cookie responses
> to the initiator's IKE_SA_INIT message, and each of those forged cookie replies will
> cause two packets to be sent: one packet from the initiator to the responder (which will
> reject those cookies), and one response from responder to initiator that includes the
> correct cookie."

`rfc/full/rfc7296.txt:1880-1890`. Appendix A quotes to "as if no cookie had been
included" and drops the rest. **Restore at least the "usually this means sending a response
containing a new cookie" clause**, because that clause is what makes the fail-open reading
safe (section 1.6), and drop the rest into the row's tag rather than the row text.

The secret-rotation permission the design relies on:

> "If a new value for <secret> is chosen while there are connections in the process of being
> initialized, an IKE_SA_INIT might be returned with other than the current
> <VersionIDofSecret>.  The responder in that case MAY reject the message by sending another
> response with a new cookie or it MAY keep the old value of <secret> around for a short
> time and accept cookies computed from either one.  The responder should not accept cookies
> indefinitely after <secret> is changed, since that would defeat part of the DoS
> protection.  The responder should change the value of <secret> frequently, especially if
> under attack."

`rfc/full/rfc7296.txt:1869-1878`.

#### What Ze does today

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| The responder's payload loop reads exactly one notify type and ignores every other | `handleSAInitRequest` | `internal/component/ike/engine/responder.go` (only `NotifySignatureHashAlgorithms`) |
| Nothing expects a cookie, so nothing can mismatch one | none exists | -- |

**Verdict: absent.** Vacuously "not violated" today, and section 1.7 says why that is not a
verdict of conformant.

### 1.6 The fail-open in `2.6-5` versus `ai/rules/evidence.md`

`ai/rules/evidence.md` requires a guard to deny on a miss, an empty set or an
error. `RFC7296-2.6-5` requires the opposite for the cookie COMPARISON: on a mismatch, do
not reject, "process the message as if no cookie had been included". The spec anticipates
this and warns the fail-open "must not be 'hardened' into a rejection"
(the rfcgate-1b RFC 7296 pilot spec).

**The two rules do not actually conflict, and the resolution is structural rather than a
carve-out.** There are two guards, not one:

| Guard | Question it answers | Behaviour on miss/empty/error |
|-------|--------------------|-------------------------------|
| `cookieRequired(ps)` | Is Ze under enough half-open pressure to demand a cookie at all? | **fails CLOSED**: any error, any unreadable state, any doubt reads `true`, so a cookie is demanded |
| `verifyCookie(data, ...)` | Does this cookie prove the sender receives at the claimed address? | **fails CLOSED**: empty data, wrong length, unknown secret version, comparison error all read `false` |
| the ACTION on `verifyCookie == false` | What happens next? | "process as if no cookie had been included", which under `cookieRequired == true` means **issue a new cookie and commit no state** |

So the mismatch path denies the resource (the half-open slot is not taken) while conforming
to the letter of `2.6-5` (the message is not rejected; it is processed as a cookie-less
IKE_SA_INIT, and the cookie-less processing of a pressured responder is "send a cookie").
**The fail-open is in the classification, never in the resource decision.** Write that
sentence into `verifyCookie`'s doc comment; it is the thing a future reader will otherwise
"harden" and break.

The one genuine fail-open: when `cookieRequired` is false, a mismatched cookie is ignored
and the exchange proceeds. That is exactly what the RFC prescribes, and it is safe because
`cookieRequired == false` means the responder has capacity.

### 1.7 `RFC7296-2.6.1-1` -- MUST NOT fail when the peer lacks the shorter exchange

#### The obligation, verbatim

> "If both peers support including the cookie in all retries, a slightly shorter exchange
> can happen.
>
>    Initiator                   Responder
>    -----------------------------------------------------------
>    HDR(A,0), SAi1, KEi, Ni -->
>                            <-- HDR(A,0), N(COOKIE)
>    HDR(A,0), N(COOKIE), SAi1, KEi, Ni  -->
>                            <-- HDR(A,0), N(INVALID_KE_PAYLOAD)
>    HDR(A,0), N(COOKIE), SAi1, KEi', Ni -->
>                            <-- HDR(A,B), SAr1, KEr, Nr
>
> Implementations SHOULD support this shorter exchange, but MUST NOT fail if other
> implementations do not support this shorter exchange."

`rfc/full/rfc7296.txt:1928-1941`. The MUST NOT is on `:1940-1941`. Appendix A
quotes the last sentence exactly and classes the row `MUST NOT`. Correct.

The paragraph that names the failure mode the MUST NOT guards:

> "If the initiator includes the cookie only in the next retry, one additional round trip
> may be needed in some cases.  An additional round trip is needed also if the initiator
> includes the cookie in all retries, but the responder does not support this.  For
> instance, if the responder includes the KEi payloads in cookie calculation, it will
> reject the request by sending a new cookie."

`rfc/full/rfc7296.txt:1921-1926`.

#### What the obligation actually requires of Ze

Read as a receiver obligation on the INITIATOR, which is where the failure can occur: Ze
retains a cookie across a subsequent retry (the SHOULD), so Ze must survive a responder that
answers that retry with a NEW cookie rather than accepting the retained one. "MUST NOT fail"
means the SA must not die; the new cookie must replace the old one and the exchange must
continue.

**The diagram at `:1933-1938` also settles the `2.6-4` / `1.2-5` interaction.** Line `:1937`
shows `KEi'` -- a CHANGED KE payload -- carried beside the retained `N(COOKIE)` and the
unchanged `Ni`. So `2.6-4`'s "all other payloads unchanged" is scoped to a cookie-only
retry; when an INVALID_KE_PAYLOAD is the cause, the KE payload changes and the nonce does
not. Without this diagram the two MUSTs read as contradictory. **Put that citation in both
rows' tags.**

#### What Ze does today

No retry loop exists (`handleSAInitResponse`, `internal/component/ike/engine/fsm.go`),
so there is nothing that can absorb a second cookie. **Verdict: absent.**

### 1.8 Why no row here is "conformant by absence"

WP-5 discharged four rows as conformant because the obligations' antecedents were
unreachable by a property the code HAS (`plan/handover/02-design-wp5.md`). That move
is not available here, for three independent reasons.

1. **`2.6-4` and `1.2-5` are unconditional receiver obligations.** They bind Ze whenever a
   PEER sends the notify. Ze cannot make a peer's behaviour unreachable. strongSwan issues
   COOKIE under load by default, and any responder with a different `dh-group` preference
   issues INVALID_KE_PAYLOAD on the first exchange.
2. **Building `2.6-4` forces the cookie machinery into existence anyway.** Once Ze can echo
   a cookie, `2.6-3`'s length bound is a real path on real attacker-controlled input.
3. **`ai/rules/rfc-compliance.md` forbids choosing the narrower option.** "Implement the
   responder half too" is the fuller answer, it is on the table, and section 3.2 shows it
   closes a real availability defect. Choosing initiator-only would be a compliance decision
   that lowers what Ze owes, which only Thomas may take.

The triage's "hold only by absence" list names `3.2-1`, `2.5-12` and `3.12-1`
(the rfcgate-1b RFC 7296 pilot spec). None of WP-4's rows is on it, and the
"genuinely unsited" table does not name one either. The triage agrees with
this section.

---

## 2. Production code -- the initiator half

### 2.1 Where the retry lives, and why

The retry belongs in `handleSAInitResponse` (`internal/component/ike/engine/fsm.go`),
not in `runInitiator`.

- It is where the notify is parsed and where the responder's chosen group is known.
- It already runs on the dispatch goroutine, which the code documents as owning a
  pre-established SA: "(re)handshake packets are handled inline on the dispatch goroutine,
  not routed to the owner loop" (`runOnce`, `internal/component/ike/engine/fsm.go`).
- It already writes the retransmit fields that a retry must reset:
  `sa.RetransmitTime` and `sa.RetransmitCount` at `internal/component/ike/engine/fsm.go`.
  The retry follows an established ownership pattern rather than inventing one.

`runInitiator`'s loop needs no change at all. It waits for `StateEstablished`
(`internal/component/ike/engine/fsm.go`) and retransmits `sa.LastSentMsg`
(`:154`); a retry that replaces `sa.LastSentMsg` and resets the timer is transparent to it.

### 2.2 New file: `internal/component/ike/engine/sa_init_retry.go`

One file, one concern (`ai/rules/go-standards.md`). `fsm.go` is already 31 KB.

| Symbol | Shape | What it does |
|--------|-------|--------------|
| `maxSAInitRetries` | const, 3 | Total IKE_SA_INIT retries per cycle, across BOTH causes. The RFC's "limit the number of cookie exchanges it tries before giving up" (`rfc/full/rfc7296.txt:1884-1885`) |
| `retrySAInit(sa *SA, cause retryCause, data []byte, tr, remote, log) bool` | func | The single retry entry point. Returns false when the retry is refused, and the caller then dies as today |
| `retryCause` | `uint8` enum, zero invalid | `retryCookie`, `retryInvalidKE`. Typed rather than a string (`ai/rules/go-standards.md`) |
| `parseInvalidKEGroup(data []byte) (ipsec.DHGroup, bool)` | func | Fail-closed parse of the 2-octet group number |

`retrySAInit` in order:

1. **Bound.** `sa.SAInitRetries++`; if it exceeds `maxSAInitRetries`, log at INFO naming the
   cause and the count, set `StateDead`, return false.
2. **Reset the responder SPI.** `sa.ResponderSPI = [8]byte{}`, and
   `table.UpdateKey(sa.ResponderSPI, [8]byte{}, sa)` back to the zero key. **Required**:
   `handleSAInitResponse` copies the header's responder SPI into the SA at
   `internal/component/ike/engine/fsm.go` BEFORE the notify switch runs, and RFC 7296
   says a non-zero responder SPI in such a response "should not" be rejected
   (`rfc/full/rfc7296.txt:1768-1769`). Without the reset, a peer that sets a non-zero
   responder SPI on its COOKIE response makes Ze send a retry whose header claims an IKE SA
   that does not exist. This is a second-order bug that only appears against a
   non-strongSwan peer, so it will not show up in the interop scenario.
3. **Apply the cause.**
   - `retryCookie`: validate `1 <= len(data) <= 64` (this is `2.6-3` on the echo path);
     `sa.Cookie = append([]byte(nil), data...)`. **Do not touch `sa.LocalNonce`,
     `sa.LocalDH` or `sa.InitiatorSPI`** -- that is `2.6-4` property 2.
   - `retryInvalidKE`: `parseInvalidKEGroup`; then the acceptance guard in 2.3; then
     `crypto.NewDHExchange(group)` into `sa.LocalDH` after `sa.LocalDH.Clear()`. **Do not
     touch `sa.LocalNonce` or `sa.Cookie`** -- that is the `KEi'` / retained-`Ni` shape of
     `rfc/full/rfc7296.txt:1937`.
4. **Rebuild and re-anchor.** `msg := buildSAInitRequest(sa, sa.IKEGroup)`;
   `sa.InitiatorSAInitMsg = msg`; `sa.LastSentMsg = msg`.
   **`sa.InitiatorSAInitMsg` is not bookkeeping.** It is the octet string the AUTH payload is
   computed over: `computeAuth`-family code reads `realMsg = sa.InitiatorSAInitMsg`
   (`internal/component/ike/engine/auth.go`) and `realMsg = sa.ResponderSAInitMsg`
   (`:58`). A retry that leaves the old bytes there makes every later IKE_AUTH fail
   authentication against a conforming peer, and the failure surfaces two messages later as
   an opaque AUTH mismatch. **This is the single easiest way to get WP-4 wrong.**
5. **Reset the timer.** `sa.RetransmitCount = 0`;
   `sa.RetransmitTime = time.Now().Add(retransmitBase)`. State stays `StateSAInitSent`.
6. **Send** via `sendWithNATT(sa, msg, tr, remote)`, matching
   `internal/component/ike/engine/fsm.go`.

### 2.3 The DH-group acceptance guard (fail-closed, and the `1.2-6` companion)

`parseInvalidKEGroup` and its caller must refuse, in this order:

| Condition | Why | `ai/rules/evidence.md` |
|-----------|-----|----------------------------------|
| `len(data) != 2` | §1.3 fixes the body: "two octets of data ... the accepted Diffie-Hellman group number in big endian order" (`rfc/full/rfc7296.txt:750-752`) | an empty or short body is a miss, so deny |
| the 16-bit value exceeds 255 | `ipsec.DHGroup` is `uint8` (`internal/component/ike/ipsec/types.go`). A silent narrowing turns group 4096 into group 0 | deny, never truncate (`ai/rules/protocol.md`) |
| `!ipsec.ValidDHGroup(g)` | `internal/component/ike/ipsec/types.go` bounds it to 1..31 | deny |
| the group is not among `sa.IKEGroup.Proposals[*].DHGroup` | **the security guard.** The notify is unauthenticated. Without this, a forged notify steers Ze onto any group it can build | deny, and log at WARN with the offered group and the configured set |
| `crypto.NewDHExchange(g)` errors | its switch covers a fixed set (`internal/component/ike/crypto/dh.go`) and returns an error otherwise | deny |

**The fourth row is what `1.2-6` is really about.** Re-proposing the full suite set stops an
attacker downgrading the CIPHER; refusing an unproposed group stops the same attacker
downgrading the GROUP. The RFC only writes the first down, because it assumes the initiator
will not build a group it never offered. Ze must be explicit. Say so in `1.2-6`'s tag.

**A zero value must not read as a valid answer.** `parseInvalidKEGroup` returns
`(ipsec.DHGroup, bool)` and the caller MUST test the bool. A bare `ipsec.DHGroup` return
would make group 0 -- "no DH" -- look like a successful parse of a truncated body.

### 2.4 `internal/component/ike/engine/initiator.go` -- `buildSAInitRequest`

Two changes, both inside the existing function, no signature change:

| Today | Change | Why |
|-------|--------|-----|
| `dhGroupID := crypto.DHGroupID(ikeGroup.Proposals[0].DHGroup)` at `:54`, used for the KE payload at `:59` | read `sa.LocalDH.GroupID` instead | `DHExchange` carries its own group (`internal/component/ike/crypto/dh.go`). Today the KE group and the actual DH key are computed independently from the same config index and can silently diverge; after a retry they WOULD diverge. Making the payload follow the key is the correct invariant regardless of this package |
| payload slice built at `:56-64` | when `len(sa.Cookie) > 0`, prepend `{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyCookie, NotificationData: sa.Cookie}}` as element 0 | `2.6-4` "as the first payload". `Message.WriteTo` emits slice order (`internal/component/ike/wire/message.go`) |
| `buildWireIKEProposals(ikeGroup)` at `:53` | **unchanged** | this IS `1.2-6`. Do not narrow the offer to the responder's group |

`buf := make([]byte, msg.Len())` at `:89` already sizes from the message, so a 64-octet
cookie needs no buffer change. The comment at `:84-88` says the length "is not remotely
influenced"; with a cookie it now is, bounded to 64 octets by 2.2 step 3. **Update that
comment** -- it is a claim about attacker influence and it stops being true.

### 2.5 `internal/component/ike/engine/fsm.go` -- the two new notify cases

Inside `handleSAInitResponse`'s payload loop, beside the existing
`NotifyNoProposalChosen` arm at `:415`:

```
NotifyCookie        -> record data, set a pending cause, break out of the loop
NotifyInvalidKEPayload -> record data, set a pending cause, break out of the loop
```

**Do not call `retrySAInit` from inside the loop.** Record the cause and act after the loop,
for two reasons. A message can legally carry several notifies, and RFC 7296 §2.6.1's
diagram (`rfc/full/rfc7296.txt:1933-1938`) shows COOKIE and INVALID_KE_PAYLOAD in the same
exchange even though not in the same message; acting mid-loop would let payload order decide
which cause wins. Precedence must be explicit: **COOKIE first, then INVALID_KE_PAYLOAD**,
because a cookie challenge means the responder committed no state and did not evaluate the
KE payload at all.

The retry must run BEFORE the completeness gate at `:450-454`, since a notify-only response
has no SA, KE or Nonce by design.

### 2.6 `internal/component/ike/engine/sa.go` -- new fields

| Field | Type | Purpose |
|-------|------|---------|
| `Cookie` | `[]byte` | the cookie to echo on the next request; nil means omit |
| `SAInitRetries` | `int` | the `maxSAInitRetries` bound |

Both are written only on the dispatch goroutine, beside `sa.RetransmitCount`
(`internal/component/ike/engine/fsm.go`), and read by `buildSAInitRequest` on the same
goroutine. No new lock. R-WP4-7 records the residual risk.

---

## 3. Production code -- the responder half

### 3.1 New file: `internal/component/ike/engine/cookie.go`

| Symbol | Shape | What it does |
|--------|-------|--------------|
| `cookieSecret` | struct: `current`, `previous [32]byte`, `version uint8`, `rotatedAt time.Time`, `mu sync.Mutex` | the rotating secret |
| `cookieRotateInterval` | const, 60s | "The responder should change the value of <secret> frequently" (`rfc/full/rfc7296.txt:1877-1878`) |
| `mintCookie(spiI [8]byte, ni []byte, ip net.IP, now time.Time) []byte` | func | `version(1) || HMAC-SHA256(secret, ni ‖ ip ‖ spiI)` truncated to 32, so **33 octets**, inside `2.6-3`'s 1..64 |
| `verifyCookie(data []byte, spiI [8]byte, ni []byte, ip net.IP, now time.Time) bool` | func | fail-closed; accepts the current secret, and the previous one for one interval |
| `firstCookieNotify(data []byte) ([]byte, bool)` | func | bounded scan of the payload chain for the first COOKIE notify |
| `cookieRequired(ps *PeerSession) bool` | func | the pressure gate |

**The construction follows the RFC's own worked example** and cites it:

> "A good way to do this is to set the responder cookie to be:
>
>    Cookie = <VersionIDofSecret> | Hash(Ni | IPi | SPIi | <secret>)
>
> where <secret> is a randomly generated secret known only to the responder and
> periodically changed and | indicates concatenation."

`rfc/full/rfc7296.txt:1838-1843`, with the rationale for each input at `:1855-1867`. Use
HMAC rather than a raw hash over a concatenation that ends in the secret; the RFC explicitly
frees the choice ("The exact algorithms and syntax used to generate cookies do not affect
interoperability and hence are not specified here", `:1832-1834`), and a raw
`Hash(data ‖ secret)` is length-extension-shaped. Compare with `hmac.Equal`, never `==`.

`verifyCookie` denies on: nil or empty `data`; `len(data) != 33`; a version octet matching
neither secret; a previous-secret match older than one rotate interval; a nil `ip`; an empty
`ni`. Every one of those is a miss, and every one denies.

### 3.2 `internal/component/ike/engine/register.go` -- where the check goes, and why it matters

**Placement is the entire security value.** In `tryResponderSAInit`
(`internal/component/ike/engine/register.go`) the check must sit AFTER
`matchResponderPeer` and BEFORE `ps.responderBusy.CompareAndSwap(false, true)`
(`:606`).

RFC 7296's rationale, which is what the placement implements:

> "Two expected attacks against IKE are state and CPU exhaustion, where the target is
> flooded with session initiation requests from forged IP addresses.  These attacks can be
> made less effective if a responder uses minimal CPU and commits no state to an SA until it
> knows the initiator can receive packets at the address from which it claims to be sending
> them."

`rfc/full/rfc7296.txt:1771-1776`.

**Defect D-2, stated precisely.** Today the CAS at `internal/component/ike/engine/register.go`
is the first thing a datagram from the configured peer address reaches. It commits that
peer's ONLY half-open slot. The slot is released by `reapStaleHandshake`
(`internal/component/ike/engine/fsm.go`) after `responderHandshakeTimeout = 30 * time.Second`
(`internal/component/ike/engine/fsm.go`). So **one spoofed UDP datagram bearing the
configured peer's source address denies that peer's IKE for 30 seconds, and a datagram every
30 seconds denies it indefinitely.** The token bucket at `:638` does not help: it is global
and generous (100/s, burst 200), and the attack needs one packet per 30 seconds.

`matchResponderPeer` narrows the attack to an off-path attacker who knows the
configured peer's address, which is not a high bar for a site-to-site VPN with published
endpoints. COOKIE closes it: the CAS is reached only by a sender that echoed a cookie bound
to its own address, which a blind spoofer cannot produce.

**This is why the responder half is in scope.** It is not "extra credit for a SHOULD"; it
repairs a reachable availability defect. `ai/rules/completion.md` applies: WP-4 is the entry
point that reaches it.

The inserted block:

1. `if !cookieRequired(ps) { fall through to the CAS unchanged }`.
2. `cookie, ok := firstCookieNotify(pkt.Data)`.
3. Extract `spiI` (already in hand at `:582`) and `Ni`. **`Ni` is a problem**: reading the
   nonce needs the payload chain. `firstCookieNotify` should return the nonce too, in one
   pass -- name it `scanSAInitPreState(data) (cookie, nonce []byte, ok bool)`.
4. `if !ok || !verifyCookie(cookie, spiI, nonce, pkt.RemoteAddr.IP, time.Now())` ->
   `sendCookieChallenge(...)`; `return true`. **No SA created, no CAS taken, no table
   insert.** This is `2.6-5`'s "process the message as if no cookie had been included;
   usually this means sending a response containing a new cookie".
5. Otherwise fall through to the CAS at `:606` unchanged.

### 3.3 `cookieRequired` -- the pressure gate, and its fail-closed reading

Two inputs, OR-ed:

| Input | Source | Meaning |
|-------|--------|---------|
| this peer's half-open slot is already taken | `ps.responderBusy.Load()` (`internal/component/ike/engine/register.go` is the writer) | a second concurrent initiation for this peer is exactly the contested case |
| a global half-open count over a threshold | a new `atomic.Int64` incremented at the CAS and decremented where `responderBusy` is released | "When a responder detects a large number of half-open IKE SAs" (`rfc/full/rfc7296.txt:1799-1800`) |

**Fails closed**: a nil `ps`, a nil remote address, or any read error returns `true` (demand a
cookie). Demanding a cookie costs a conforming peer one round trip and costs an attacker the
whole attack, so `true` is the safe default in every direction.

**Configurable, and defaulting to on.** Add a YANG leaf under the IPsec container
(`ai/rules/config.md`: an operator would tune this during capacity planning, and it
must appear in `show configuration`). `cookie-threshold`, `type uint32`, `units` not needed
(a count), `default 1`. Value 0 means never demand a cookie. Naming per
`ai/rules/config.md`; the env override follows the YANG path.

### 3.4 `internal/component/ike/engine/responder.go` -- `sendCookieChallenge`

`sendSAInitNotify` takes an `*SA`, and the whole point of a cookie challenge is
that no SA exists. A sibling is needed:

`sendCookieChallenge(tr *transport.UDPTransport, remote *net.UDPAddr, spiI [8]byte, cookie []byte, log *slog.Logger)`

- Header: `InitiatorSPI: spiI`, **`ResponderSPI: [8]byte{}`** -- "When the IKE_SA_INIT
  exchange does not result in the creation of an IKE SA due to INVALID_KE_PAYLOAD,
  NO_PROPOSAL_CHOSEN, or COOKIE, the responder's SPI will be zero also in the response
  message" (`rfc/full/rfc7296.txt:1765-1767`).
- `ExchangeIKESAInit`, `FlagResponse`, `MessageID: 0`.
- One `PayloadNotify{NotifyMsgType: wire.NotifyCookie, NotificationData: cookie}`.
- Encode with `CheckedWriteTo` into a 512-octet buffer, matching `sendSAInitNotify:344-351`,
  and drop rather than truncate on error.
- **Assert `1 <= len(cookie) <= 64` before sending** and drop with a WARN naming the length
  if not. `mintCookie` returns 33, so this can only fire on a code change -- which is
  precisely what makes it the `2.6-3` guard rather than a comment.

Refactor `sendSAInitNotify` to delegate to a shared `sendSAInitNotifyRaw(tr, remote, spiI,
spiR, notifyType, data, peerName, log)`, so the existing INVALID_KE and NO_PROPOSAL paths and
the cookie path share one encoder and one bound. `TestSendSAInitNotifyBytesUnchanged`
(`internal/component/ike/engine/overflow_test.go`) already pins those bytes and will
catch a refactor that changes them.

### 3.5 Metrics

`internal/component/ike/engine/metrics.go` holds only gauges refreshed by `Update()`
(`:44-80`). Cookie activity is event-shaped, so use `metrics.CounterVec`
(`internal/core/metrics/metrics.go`, `:60`):

| Metric | Labels | Incremented at |
|--------|--------|----------------|
| `ze_ipsec_cookie_challenges_total` | `peer` | `sendCookieChallenge` |
| `ze_ipsec_cookie_verify_failures_total` | `peer` | the `verifyCookie == false` branch |
| `ze_ipsec_sa_init_retries_total` | `peer`, `cause` | `retrySAInit` step 1 |

The third is the operator's signal for a forged-notify flood against the initiator, which is
the attack §2.6 describes at `rfc/full/rfc7296.txt:1886-1890`. Document all three in the
subsystem telemetry page per `ai/rules/writing.md` row 14.

---

## 4. Bounding every unauthenticated emission

The brief requires this section to be explicit. Both surfaces are unauthenticated, and both
are answered below by construction rather than by a limiter alone.

### 4.1 The responder's COOKIE challenge

| Property | Answer | Evidence |
|----------|--------|----------|
| **Amplification** | **Impossible. The response is strictly smaller than the request.** A cookie challenge is 28 (header) + 4 (generic) + 4 (notify) + 33 (cookie) = **69 octets**. It answers an IKE_SA_INIT carrying SA + KE + Nonce; with MODP-2048 the KE payload alone is 256 octets of public value, so the request exceeds 350 octets. The ratio is below 0.2 and can never exceed 1 | `mintCookie` returns 33; `PayloadNotify.Len` = `4 + spiLen + len(data)` (`internal/component/ike/wire/payload_notify.go`); `padBigInt(pub, modp2048Len)` (`internal/component/ike/crypto/dh.go`) |
| **Reflection** | The challenge goes only to `pkt.RemoteAddr`, and only after `matchResponderPeer` matched that address against a CONFIGURED peer (`internal/component/ike/engine/register.go`, `:566-573`). An attacker cannot aim it at a third party, because the only reachable destination is an address the operator already configured | `matchResponderPeer`, `internal/component/ike/engine/register.go` |
| **Rate** | Governed by the existing inbound token bucket, since a challenge is emitted at most once per accepted datagram: 100/s sustained, burst 200, per socket | `dispatchInbound`, `internal/component/ike/engine/register.go`, `:648`; `inboundRateLimiter.allow`, `:425` |
| **CPU** | One HMAC-SHA256 over ~60 octets plus a bounded payload scan. No allocation of payload objects (section 4.3), no DH, no SA. Strictly less work than today's path, which builds an `SA`, a DH keypair and a table entry | `newResponderSA` (`internal/component/ike/engine/responder.go`) is what the check now skips |
| **State** | **Zero.** No SA, no CAS, no table entry, no per-address table. The secret is one process-wide struct, and the cookie is recomputed rather than remembered | RFC's stateless design, `rfc/full/rfc7296.txt:1830-1832` |

**A dedicated outbound limiter is deliberately NOT added.** The inbound bucket already
bounds the emission one-for-one, and a second bucket would be a new failure mode with no new
guarantee. If a reviewer disagrees, the cheap answer is to reuse `newInboundRateLimiter`
(`internal/component/ike/engine/register.go`) with a tighter rate for challenges only;
say so at review rather than adding it speculatively.

### 4.2 The initiator's retry, and the two-Ze loop

**The packet loop the brief asks about is real and is closed by three independent bounds.**
Consider Ze-as-initiator against Ze-as-responder, or a forged-notify flood
(`rfc/full/rfc7296.txt:1886-1890`).

| Bound | Value | Where |
|-------|-------|-------|
| Retries per cycle, across BOTH causes | `maxSAInitRetries = 3` | `retrySAInit` step 1. **Shared counter, not per-cause**, so alternating COOKIE and INVALID_KE_PAYLOAD cannot pump it |
| Cycles per peer | `maxRetransmissions`, then `errTimeout` | `runInitiator`, `internal/component/ike/engine/fsm.go` |
| Delay between cycles | exponential, capped at `reconnectMaxDelay` | `reconnectDelay`, `internal/component/ike/engine/fsm.go` |

A retry is sent only in DIRECT response to a received notify, never on a timer, so the
initiator's send rate can never exceed the rate of notifies it receives, which the peer's own
inbound bucket bounds. **The exchange is strictly convergent**: each retry either succeeds,
is refused by a guard in 2.3, or burns one of three tokens.

**Why the shared counter matters.** A per-cause counter permits an unbounded
COOKIE / INVALID_KE / COOKIE / INVALID_KE alternation, which is precisely the shape §2.6.1
describes when the responder folds KEi into its cookie calculation
(`rfc/full/rfc7296.txt:1924-1926`). Ze must converge or give up, never oscillate. Write that
sentence beside the constant.

### 4.3 The pre-state payload scan

`scanSAInitPreState` runs on unauthenticated input before any state exists, so it must not
be the DoS it prevents. `wire.Message.ReadFrom` is the wrong tool here: it allocates a
payload object per payload, and `PayloadNotify.ReadFrom` allocates its data slice
(`internal/component/ike/wire/payload_notify.go`, `:143`). The scan instead walks the
generic payload headers over the raw slice and copies only the cookie and the nonce, with
four bounds, each denying rather than continuing:

| Bound | Value | Reason |
|-------|-------|--------|
| max payloads walked | 32 | a chain of thousands of 4-octet payloads is a CPU loop |
| min payload length | 4 | a zero or sub-header length is an infinite loop, and this is the one that matters |
| offset must strictly advance | enforced | belt and braces on the previous row |
| max cookie copied | 64 | `2.6-3` at the input boundary; a longer one is a mismatch, not a truncation |

Fuzz it. `ai/rules/testing.md` makes a fuzz target mandatory for wire-format parsing of external
input, and this is external input reaching a hand-rolled walker.

---

## 5. Files changed

| File | Change | New? |
|------|--------|------|
| `internal/component/ike/engine/cookie.go` | secret, mint, verify, pre-state scan, pressure gate | **new** |
| `internal/component/ike/engine/sa_init_retry.go` | `retrySAInit`, `retryCause`, `parseInvalidKEGroup`, bound | **new** |
| `internal/component/ike/engine/fsm.go` | two notify cases in `handleSAInitResponse`, retry before the completeness gate | no |
| `internal/component/ike/engine/initiator.go` | `buildSAInitRequest` reads `sa.LocalDH.GroupID`, prepends the cookie; comment at `:84-88` corrected | no |
| `internal/component/ike/engine/responder.go` | `sendCookieChallenge`, `sendSAInitNotifyRaw` extraction | no |
| `internal/component/ike/engine/register.go` | the pre-CAS cookie gate in `tryResponderSAInit`; half-open counter | no |
| `internal/component/ike/engine/sa.go` | `Cookie`, `SAInitRetries` | no |
| `internal/component/ike/engine/metrics.go` | three counters | no |
| `internal/component/ike/ipsec/config.go` + the IPsec YANG | `cookie-threshold` leaf, parse, validate | no |
| `rfc/short/rfc7296.md` | six checklist rows (section 11) | no |
| `ai/RFC-REQUIREMENTS.md` | regenerated by `make ze-rfc-index-update` | no |

**Nine production files, two of them new.** The spec's phase list estimated three
(the rfcgate-1b RFC 7296 pilot spec), written before the rows were mapped to
producers. The extra six are `register.go` (which the estimate missed entirely, and which is
where the security value lives), `initiator.go`, `sa.go`, `metrics.go` and the config pair.

---

## 6. The tagged tests

Home: `internal/component/ike/engine/rfc7296_cookie_test.go` (new) for the four §2.6 rows,
and `internal/component/ike/engine/rfc7296_kegroup_test.go` (new) for the two §1.2 rows.
Two files rather than one, because the harnesses differ: the §1.2 rows are driven
synchronously through `autSAInitPair`, and `2.6-5` needs `tryResponderSAInit`, which needs a
registered `PeerSession`.

**Existing harnesses to reuse, never to reinvent:**

| Harness | `file:line` | Use for |
|---------|-------------|---------|
| `autSAInitPair(t, auth)` | `internal/component/ike/engine/rfc7296_auth_test.go` | a pair stopped exactly after IKE_SA_INIT. The natural place to inject a notify |
| `establishPSK(t)` | `internal/component/ike/engine/responder_test.go` | a full handshake, for the "retry still authenticates" assertion |
| `parseMsg(t, raw)` | `internal/component/ike/engine/responder_test.go` | decode a built message for payload-order assertions |
| `icyRunCycle(t, rekey)` / `icyFarEnd` | `internal/component/ike/engine/initiator_cycle_test.go`, `:20` | the goroutine-driven cycle with a stubbed `afterFunc`. **The only harness that exercises the retry against the real `runInitiator` loop** |
| the two-socket UDP pattern | `internal/component/ike/engine/overflow_test.go` | `sendCookieChallenge` on a real socket |

### 6.1 The pairs

| Row | Positive | Negative |
|-----|----------|----------|
| `1.2-5` | `TestKegInitiatorRetriesOnInvalidKEPayload`. Drive `autSAInitPair`, feed the initiator a hand-built INVALID_KE_PAYLOAD response naming a group the config offers. Assert: state is `StateSAInitSent` not `StateDead`; a NEW request was built; its KE payload names the responder's group; `sa.LocalDH.GroupID` matches it | `TestKegInitiatorRefusesUnofferedGroup`. Feed a group Ze never proposed, then a 1-octet body, then a 3-octet body, then group 0, then a 16-bit value above 255. **Each must leave the SA dead and send nothing.** This is the discriminating half: it proves the retry is a guarded decision rather than obedience to an unauthenticated packet |
| `1.2-6` | `TestKegRetryReproposesEveryConfiguredSuite`. Configure three proposals with distinct ciphers and two distinct groups. Assert the RETRY's SA payload carries all three, with the same numbering as the first attempt | `TestKegRetryOfferIsNotNarrowedByTheNotify`. Assert the retry's proposal set is byte-identical to the first attempt's SA payload. Then assert the KE payload DID change. Together these say "the group moved and nothing else did", which no single assertion says |
| `2.6-3` | `TestCkeMintedCookieIsWithinTheLengthBound`. Sweep 256 mints across varying SPIi, Ni and IP; assert every length is in 1..64. Then assert `sendCookieChallenge` refuses a 0-octet and a 65-octet cookie and sends nothing | `TestCkeEchoedCookieIsBoundedToo`. Feed the initiator a COOKIE response with a 65-octet body and assert NO retry is sent. Then feed a 64-octet body and assert a retry IS sent. The boundary pair proves the bound is `<= 64`, not `< 64` (`ai/rules/testing.md`, boundary testing) |
| `2.6-4` | `TestCkeRetryCarriesCookieFirstAndNothingElseChanged`. Assert, on the retry: payload 0 is a `PayloadNotify` with `NotifyMsgType == wire.NotifyCookie` and data equal to the received bytes; the Nonce payload is byte-identical to the first attempt; the KE payload is byte-identical; the header's initiator SPI is unchanged; `MessageID == 0` | `TestCkeCookieIsAbsentWithoutAChallenge`. Assert the FIRST request carries no COOKIE payload at all. Without this, "payload 0 is a cookie" could pass over a builder that always emits one, and the peer would see a cookie it never issued |
| `2.6-5` | `TestCkeMismatchedCookieIsIgnoredNotRejected`. With `cookieRequired` forced true, drive `tryResponderSAInit` with a wrong cookie. Assert: a NEW cookie challenge is sent (not a NO_PROPOSAL_CHOSEN, not silence); `ps.responderBusy` is still false; the SA table is empty; the function returned true | `TestCkeValidCookieReachesTheHandshake`. The same input with a cookie from `mintCookie` must take the CAS and create the SA. **This is what stops the positive being vacuous**: without it, a `tryResponderSAInit` that rejected everything would pass the positive |
| `2.6.1-1` | `TestCkeSecondCookieReplacesTheFirstWithoutFailing`. Send COOKIE `X`; assert a retry carrying `X`. Send COOKIE `Y` in response to that retry; assert a second retry carrying `Y`, still `StateSAInitSent`, nonce and DH unchanged throughout | `TestCkeCookieAndInvalidKECombineWithoutFailing`. Reproduce `rfc/full/rfc7296.txt:1933-1938` exactly: COOKIE, then INVALID_KE_PAYLOAD. Assert the third request carries BOTH the retained cookie AND the corrected group, with the nonce still unchanged. Then assert the fourth notify is REFUSED by `maxSAInitRetries` and the SA dies, which proves the tolerance is bounded rather than unbounded |

**One integration test beside the six pairs, untagged.**
`TestKegRetriedSAInitStillAuthenticates`: drive `icyRunCycle` with an `icyFarEnd` that
answers the first IKE_SA_INIT with INVALID_KE_PAYLOAD, then completes normally. Assert the
SA reaches `StateEstablished`. **This is the only test that catches the
`sa.InitiatorSAInitMsg` bug of section 2.2 step 4**, because AUTH is computed over that field
(`internal/component/ike/engine/auth.go`) and every payload-shape assertion above passes
with the stale value.

### 6.2 Mutations -- each must redden its named test

| # | Mutation | Site | Must redden |
|---|----------|------|-------------|
| 1 | delete the `NotifyInvalidKEPayload` case | `handleSAInitResponse` | `1.2-5` positive |
| 2 | drop the "group was proposed" check | `retrySAInit` / `parseInvalidKEGroup` | `1.2-5` negative ONLY. If the positive also reddens, the negative is not testing what it claims |
| 3 | `parseInvalidKEGroup` returns `ipsec.DHGroup(data[1])` with no length test | same | `1.2-5` negative (the 1-octet and 3-octet cases) |
| 4 | the retry calls `buildWireIKEProposals` with only the responder's group | `buildSAInitRequest` | `1.2-6` positive AND negative |
| 5 | `mintCookie` returns 65 octets | `mintCookie` | `2.6-3` positive |
| 6 | the echo path drops its length test | `retrySAInit` step 3 | `2.6-3` negative |
| 7 | the cookie is appended rather than prepended | `buildSAInitRequest` | `2.6-4` positive |
| 8 | the retry calls `GenerateNonce` for a fresh `sa.LocalNonce` | `retrySAInit` | `2.6-4` positive. **Run this one explicitly.** It is the mutation a plausible wrong implementation actually contains |
| 9 | the retry calls `crypto.NewDHExchange` on the COOKIE path too | `retrySAInit` | `2.6-4` positive (the KE-unchanged assertion) |
| 10 | `buildSAInitRequest` always prepends a cookie, even when nil | `buildSAInitRequest` | `2.6-4` negative |
| 11 | `verifyCookie` returns false and the caller sets `StateDead` instead of re-challenging | `tryResponderSAInit` | `2.6-5` positive. This is the "hardened into a rejection" error the spec warns about |
| 12 | `verifyCookie` returns true unconditionally | `verifyCookie` | `2.6-5` positive |
| 13 | `verifyCookie` returns false unconditionally | `verifyCookie` | `2.6-5` negative |
| 14 | the cookie gate is moved AFTER the CAS | `tryResponderSAInit` | `2.6-5` positive (the `responderBusy` assertion). **The whole security value is in this line's position, so this mutation is mandatory** |
| 15 | `sa.Cookie` is cleared before the second retry | `retrySAInit` | `2.6.1-1` positive |
| 16 | `maxSAInitRetries` becomes unbounded | `retrySAInit` step 1 | `2.6.1-1` negative |
| 17 | `sa.InitiatorSAInitMsg` is not re-anchored on retry | `retrySAInit` step 4 | `TestKegRetriedSAInitStillAuthenticates` ONLY. Confirms every other test is blind to it |
| 18 | `sa.ResponderSPI` is not reset on retry | `retrySAInit` step 2 | a dedicated assertion in `2.6-4`'s positive: build the challenge with a NON-zero responder SPI and assert the retry's header carries zero |

Mutations 8, 14 and 17 are the three the design most expects an implementer to introduce.
**Run all three explicitly and paste the red output**, per
`ai/rules/testing.md` (mutation-verify).

### 6.3 Functional and interop

| Test | Location | Proves |
|------|----------|--------|
| `ipsec-cookie-challenge.ci` | `test/ipsec/` (does not exist today) | Ze-to-Ze over loopback: with `cookie-threshold 1` the responder challenges and the exchange completes on retry. Follow the existing pattern exactly -- `cmd=background` responder plus `cmd=foreground` initiator, `option=env:var=ze_test_ike_dataplane:value=noop`, `option=env:var=ze_test_ike_port:value=$PORT2`. **No `option=needs-linux`**: no `test/ipsec/*.ci` uses it, because the noop dataplane avoids the privilege entirely. Assert on `show vpn ipsec sa` reaching an established state with a real negotiated cipher, as `test/ipsec/ipsec-sa-installed.ci` does |
| `12-invalid-ke-retry` | `test/ipsec-interop/scenarios/` | Ze as initiator configured `dh-group 14` first against a strongSwan proposing only `modp3072`, so strongSwan sends INVALID_KE_PAYLOAD and Ze must retry and establish. **AC-14** (the rfcgate-1b RFC 7296 pilot spec) |
| `18-cookie-challenge` | `test/ipsec-interop/scenarios/` | strongSwan as initiator against a Ze responder with `cookie-threshold 1`, proving a real third-party initiator accepts Ze's cookie and completes |

Scenario mechanics, confirmed: discovery is directory-based -- `test/ipsec-interop/run.py`
runs every subdirectory of `scenarios/` that contains a `check.py`, in sorted order, so there
is no list file to update. Each scenario is three files (`ze.conf`, `swanctl.conf`,
`check.py`) and `check.py` exposes `def check():` importing helpers from
`test/ipsec-interop/lab.py`. The make target is `make ze-interop-ipsec-test`, with
`IPSEC_INTEROP_SCENARIO=<name>` to select one (`mk/test-integration.mk`).

**A caution on `12`'s discriminating power** (`ai/rules/interop-and-goal-validation.md`,
vacuity traps). Assert on the tunnel establishing, and ALSO assert that strongSwan's log
contains its INVALID_KE_PAYLOAD emission. Without the second assertion, a scenario whose
strongSwan config happens to accept `modp2048` passes without the retry ever running, and
reverting `retrySAInit` leaves it green.

**A caution on `18`.** strongSwan issues cookies under its own load policy, so a scenario
that waits for strongSwan to become loaded is nondeterministic. Drive it the other way:
Ze is the RESPONDER with `cookie-threshold 1`, so Ze challenges every initiation
deterministically and the assertion is on strongSwan completing anyway.

---

## 7. Id allocation

`check_id_allocation` (`scripts/dev/rfc_requirements.py`, refusal at `:498-506`) refuses
a new id whose ordinal is at or below its SECTION's high-water mark, computed by
`high_water` from the COMMITTED HEAD ids. `_head_of` keys on the section
STRING, so `RFC7296-2.6` and `RFC7296-2.6.1` are DISTINCT scopes. A section with no mark is
skipped outright (`:497` `if mark is None: continue`).

### Marks measured 2026-07-31

`rfc/short/rfc7296.md` is CLEAN (`git status --porcelain` reports nothing for it), so HEAD
and the working tree agree and no staged row can move a mark under this design.

| Section | HEAD ids | Mark | Appendix A ordinal | Verdict |
|---------|----------|------|--------------------|---------|
| `RFC7296-1.2` | -1, -2, -3, -4 | **4** | `1.2-5`, `1.2-6` | **both ALLOWED as written** (5 > 4, 6 > 4) |
| `RFC7296-2.6` | -1, -2 | **2** | `2.6-3`, `2.6-4`, `2.6-5` | **all three ALLOWED as written** (3, 4, 5 > 2) |
| `RFC7296-2.6.1` | none | **none** | `2.6.1-1` | **ALLOWED**: no mark, so the check skips the section entirely |

**WP-4's six Appendix A ordinals are all legal exactly as written. No renumbering is
needed.** This is the first package in this spec for which that is true; WP-5 had to
renumber three of four (`plan/handover/02-design-wp5.md`).

### The contiguity warning still applies

No other package claims a §1.2, §2.6 or §2.6.1 ordinal today. Appendix A's §1.2 rows are
`1.2-1` through `1.2-6`, and `-1` through `-4` are already in HEAD; its §2.6 rows are `2.6-1`
through `2.6-5`, and `-1`, `-2` are already in HEAD. So WP-4 is the sole claimant in all
three sections, and no landing order can strand an ordinal.

**But that is a fact about today, and three renumberings have already cost this spec time**
(the rfcgate-1b RFC 7296 pilot spec, `:1399-1414`, `:1416-1438`). The
re-triage's new WP-7 carries 10 rows where this design covers 6, and the extra rows are very
likely §1.3 and §3.4 (section 10). **A §1.2 row landing from another package before WP-4
would push `1.2-5` and `1.2-6` up.** Recompute at the moment of landing, per section:

    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-1\.2-[0-9]*' | sort -V | tail -1
    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-2\.6-[0-9]*' | sort -V | tail -1

**Land all six rows in ONE commit**, in Appendix A ordinal order. That is not a hard
requirement of the gate, but it makes the three sections' marks move once, and it keeps the
package's ledger regeneration to a single `make ze-rfc-index-update`.

**`2.6.1` is a first enrolment for that section.** It creates the mark, and every later
§2.6.1 row must clear it. Nothing else in Appendix A sits in §2.6.1, so there is no
competition, but the row's `(§2.6.1)` citation must be exact: `parse_checklist_line`
validates that the id's section segment agrees with the citation.

---

## 8. What must NOT break

| Invariant | Why it is at risk | The guard |
|-----------|-------------------|-----------|
| **`RFC7296-1.3-2`'s responder half** (`respondIKERekey` sends INVALID_KE_PAYLOAD on a rekey group mismatch, `internal/component/ike/engine/rekey.go`, emission at `:548`) | WP-4 touches the initiator's CONSUMPTION of the same notify. An implementer who routes the CREATE_CHILD_SA response through `retrySAInit` would retry an exchange the RFC does not require retrying | `TestNegRekeyRejectsMismatchedKEGroup` (`internal/component/ike/engine/rfc7296_negotiation_test.go`). **Scope `retrySAInit` to `ExchangeIKESAInit` only**, and say so in `1.2-5`'s tag |
| **`RFC7296-3.4-1`** (MODP public values are padded to the modulus length; its negative was repaired to `TestRFC7296MODPShortPublicValueIsRefusedOnReceipt`, `internal/component/ike/crypto/rfc7296_dh_test.go`) | the retry builds a NEW `DHExchange` in a different group. If the retry ever hand-built a public value instead of calling `crypto.NewDHExchange`, the pad would be lost | `retrySAInit` calls `crypto.NewDHExchange` (`internal/component/ike/crypto/dh.go`) and nothing else. Both `3.4-1` tests stay green |
| **`RFC7296-2.6-1` and `2.6-2`** (SPI pair identity and uniqueness) | the retry resets `sa.ResponderSPI` and calls `table.UpdateKey` | `TestSPIsAreUniqueIdentifiers` (`internal/component/ike/engine/rfc7296_header_test.go`) and `TestHeaderRoundtrip` (`internal/component/ike/wire/header_test.go`) |
| **`sendSAInitNotify`'s exact bytes** | section 3.4 extracts a shared raw sender under it | `TestSendSAInitNotifyBytesUnchanged` (`internal/component/ike/engine/overflow_test.go`) pins them byte-for-byte, and `TestSendSAInitNotifyOversizedRejected` pins the drop |
| **AC-3 / AC-6 / AC-7 responder concurrency** (one half-open handshake per peer; a fresh IKE_SA_INIT beside an established SA is accepted in parallel; the established SA is untouched by an unauthenticated message) | the cookie gate is inserted into exactly that code path, before the CAS | `TestResponderKeepsOldSAOnUnauthenticatedInit`, `TestResponderSupersedesOnAuthenticatedInit`, `TestResponderAcceptsReinitAfterStaleSA`, `TestRunResponderAcceptsInboundAndBounds` (all `internal/component/ike/engine/responder_test.go`). **With `cookie-threshold` defaulting to 1, `ps.responderBusy.Load()` is one of `cookieRequired`'s inputs, so these tests now traverse the challenge path.** They must either set the threshold to 0 or supply a valid cookie. This is the largest test-compatibility surface in the package |
| **The eight existing `test/ipsec/*.ci`** | they run two real daemons; if the responder demands a cookie the initiator cannot produce, every one hangs | the initiator half lands in the SAME commit as the responder half, so Ze-to-Ze converges. **Never land the responder half alone** |
| **All eleven `test/ipsec-interop/scenarios/`** | `07-responder-psk`, `08-responder-eap-mschapv2`, `09-responder-ike-rekey`, `11-responder-accepts-reinit` drive strongSwan as initiator against a Ze responder | strongSwan implements COOKIE, so these should pass with `cookie-threshold 1`. **Verify, do not assume**: run `make ze-interop-ipsec-test` before and after |
| **`RFC7296-1.2-1`** (the initial exchange is exactly four messages, first pair unencrypted) | a cookie exchange adds two messages before the four | RFC 7296 shows exactly that (`rfc/full/rfc7296.txt:1809-1821`) and says the extra pair "do not affect any initiator or responder state except for communicating the cookie" (`:1823-1824`). `TestInitialExchangeEncryptionBoundary` (`internal/component/ike/engine/rfc7296_test.go`) asserts the boundary, not the count. **Confirm it stays green**; if it counts messages, its tag needs the §2.6 citation |

---

## 9. Risks

| Id | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-WP4-1 | **`sa.InitiatorSAInitMsg` is not re-anchored, so every retried exchange fails AUTH two messages later.** The failure is opaque and looks like a PSK problem | every payload-shape test is green and `12-invalid-ke-retry` fails at IKE_AUTH | `TestKegRetriedSAInitStillAuthenticates` (6.1) plus mutation 17. **The single highest-probability defect in this package** |
| R-WP4-2 | **The responder half is landed without the initiator half, and every Ze-to-Ze test hangs** | `test/ipsec/*.ci` time out | One commit for both halves. Section 8 |
| R-WP4-3 | **`cookie-threshold` defaulting to 1 breaks four existing responder unit tests and four interop scenarios** | `responder_test.go` reds immediately | Expected and manageable, but budget for it. Each test either sets the threshold to 0 or mints a cookie. **Do not "fix" it by defaulting the threshold to 0**: that ships the feature disabled and leaves D-2 open |
| R-WP4-4 | **`2.6-5` is "hardened" into a rejection by a later reviewer**, breaking conformance | `TestCkeMismatchedCookieIsIgnoredNotRejected` reddens | Mutation 11 is in the table for exactly this. Section 1.6's three-guard reading goes in `verifyCookie`'s doc comment, not only in this document |
| R-WP4-5 | **The pre-state scan is a DoS.** A hand-rolled walker over unauthenticated input with a zero-length payload is an infinite loop | none until a flood | The four bounds in 4.3, the strict-advance check, and a fuzz target. `ai/rules/testing.md` makes the fuzz target mandatory |
| R-WP4-6 | **The interop scenario passes without the retry ever running**, because strongSwan's default proposal happens to include Ze's first group | `12-invalid-ke-retry` green with `retrySAInit` reverted | Assert on strongSwan's INVALID_KE_PAYLOAD log line as well as on establishment. Revert-and-confirm-red before claiming the scenario is evidence (`ai/rules/interop-and-goal-validation.md`) |
| R-WP4-7 | **A data race on the new `SA` fields.** `sa.Cookie` and `sa.SAInitRetries` are written on the dispatch goroutine and `runInitiator`'s goroutine reads `sa.RetransmitCount` beside them | `go test -race` on the engine package | The pattern is pre-existing (`handleSAInitResponse` already writes `sa.RetransmitTime` at `internal/component/ike/engine/fsm.go`), so WP-4 does not introduce it. **Run the engine package under `-race` with `-count=20`** and, if it fires, fix it rather than record it (`ai/rules/completion.md`). `make ze-unit-reactor-test-race` is BGP-only and will not cover this |
| R-WP4-8 | **Engine line numbers move under a concurrent agent.** `msgid.go` and `responder.go` carry a 12:2x mtime today | a tag cites a line holding different code | Every citation here names its function. Re-read before quoting a line into a tag |
| R-WP4-9 | **An implementer routes the CREATE_CHILD_SA INVALID_KE_PAYLOAD through `retrySAInit`**, inventing a retry the RFC does not require | `TestNegRekeyRejectsMismatchedKEGroup` behaviour changes | Scope `retrySAInit` to `ExchangeIKESAInit`. Section 8, row 1 |
| R-WP4-10 | **`maxSAInitRetries` is made per-cause**, permitting an unbounded COOKIE/INVALID_KE oscillation between two Ze instances | none; the loop is slow and looks like flapping | One shared counter. Mutation 16, and the sentence beside the constant (4.2) |
| R-WP4-11 | **The nonce is regenerated on the cookie retry**, so the responder sees a different Ni and the cookie it minted over the old Ni never verifies. Two conforming implementations then loop until the bound | `18-cookie-challenge` needs three round trips instead of two | Mutation 8, run explicitly. `2.6-4`'s positive asserts byte-identity, not equivalence |
| R-WP4-12 | **`cookie-threshold` is added as an env var rather than a YANG leaf** | it never appears in `show configuration` or a config backup | `ai/rules/config.md`: an operator tunes this during capacity planning, so the default answer is YANG. Section 3.3 |

---

## 10. Items for the owner

`ai/rules/rfc-compliance.md` reserves a compliance judgement to Thomas, and voids every
earlier answer pointing away from full compliance. Three items qualify. **None asks
permission to do less; all three propose doing more, and 10.1 exists because the scope grew
past the phase list's estimate.**

### 10.1 The responder half is in scope, and it costs more than the phase list assumed

The phase list estimates three files and names `engine/fsm.go`, `engine/cookie.go` and
`engine/responder.go` (the rfcgate-1b RFC 7296 pilot spec). This design needs nine,
plus a YANG leaf, because the cookie CHECK belongs in `tryResponderSAInit`
(`internal/component/ike/engine/register.go`) rather than in `responder.go`, and because
defect D-2 is only closed if it sits before the CAS at `:606`.

**What is being asked:** confirmation that the responder half stays in, at that cost.

**Why the narrower option is not on the table for me to pick.** Initiator-only would satisfy
`1.2-5`, `1.2-6`, `2.6-4` and `2.6.1-1` and would leave `2.6-3` and `2.6-5` provable only by
absence, which is the expiring-negative shape this spec has already recorded twice
(`:1869-1889`). It would also leave D-2 open. That is a decision that lowers what Ze owes,
so it is Thomas's, not mine.

### 10.2 The §2.6 SHOULD is unextracted, and its absence weakens two rows

RFC 7296 §2.6 opens:

> "When a responder detects a large number of half-open IKE SAs, it SHOULD reply to
> IKE_SA_INIT requests with a response containing the COOKIE notification."

`rfc/full/rfc7296.txt:1799-1801`. **No Appendix A row carries it**, so `rfc/short/rfc7296.md`
has no requirement corresponding to the only sentence that says WHEN Ze should issue a
cookie. `2.6-3` and `2.6-5` both presuppose it.

`ai/rules/rfc-compliance.md` names this exact pattern: "A `{not-applicable}` whose reason is
'ze has no X producer at all' -- that admission is often the violation of a separate MUST
requiring X to exist", and requires the checklist row to be added rather than the gate's
silence taken for conformance. The SHOULD is not a MUST, so `check_enrolment` does not gate
it; but a §2.6 whose trigger sentence is unextracted is the same shape.

**What is being asked:** whether to add a SHOULD-level row for the trigger sentence. **Cost:
near zero** -- the code lands anyway as `cookieRequired` (section 3.3), and the row would be
proven by `TestCkeMismatchedCookieIsIgnoredNotRejected`'s harness with the threshold varied.

### 10.3 The new WP-7 carries 10 rows and this design covers 6

The re-triage's WP-7 is "COOKIE, DH-group retry, KE payload agreement", 10 rows
(the rfcgate-1b RFC 7296 pilot spec). The brief names 6. The four most likely
extras are `RFC7296-1.3-2` (already `impl-untested` and already gated), `3.4-1` (already
gated), `3.4-2` ("This Diffie-Hellman Group Num MUST match a Diffie-Hellman group specified
in a proposal in the SA payload that is sent in the same message", `:1202`) and `3.4-3` ("If
none of the proposals in that SA payload specifies a Diffie-Hellman group, the KE payload
MUST NOT be present", `:1203`).

**`3.4-2` is the SENDER-side twin of `1.2-6` and this design already satisfies it.** Section
2.4 makes the KE payload's group follow `sa.LocalDH.GroupID`, and section 2.3 refuses any
group Ze did not propose, so the retry's KEi group is necessarily in its own SA payload.
Folding `3.4-2` into WP-4 costs one more assertion on an existing test, not a new harness.

**What is being asked:** whether to fold `3.4-2` (and, if cheap, `3.4-3`) into this package.
§3.4's mark is 1, so `3.4-2` and `3.4-3` are legal as written. This is strictly MORE work in
one package, so `ai/rules/completion.md` does not require permission; it is raised here only
because it changes what the commit claims to close. Raise it together with 10.1.

---

## 11. Summary rows to add to `rfc/short/rfc7296.md`

Land them in section order: the two §1.2 rows after `RFC7296-1.2-4`
(`rfc/short/rfc7296.md`), the three §2.6 rows after `RFC7296-2.6-2`, and the
§2.6.1 row after them. Ordinals are the Appendix A ones, which section 7 proves legal;
recompute the two marks at landing.

    - [ ] [RFC7296-1.2-5] [MUST] If the initiator guesses wrong, the responder will respond
      with a Notify payload of type INVALID_KE_PAYLOAD indicating the selected group. In this
      case, the initiator MUST retry the IKE_SA_INIT with the corrected Diffie-Hellman group
      (§1.2)

    - [ ] [RFC7296-1.2-6] [MUST] The initiator MUST again propose its full set of acceptable
      cryptographic suites because the rejection message was unauthenticated and otherwise an
      active attacker could trick the endpoints into negotiating a weaker suite than a
      stronger one that they both prefer (§1.2)

    - [ ] [RFC7296-2.6-3] [MUST] The data associated with this notification MUST be between 1
      and 64 octets in length (inclusive) (§2.6)

    - [ ] [RFC7296-2.6-4] [MUST] If the IKE_SA_INIT response includes the COOKIE
      notification, the initiator MUST then retry the IKE_SA_INIT request, and include the
      COOKIE notification containing the received data as the first payload, and all other
      payloads unchanged (§2.6)

    - [ ] [RFC7296-2.6-5] [MUST] When one party receives an IKE_SA_INIT request containing a
      cookie whose contents do not match the value expected, that party MUST ignore the
      cookie and process the message as if no cookie had been included; usually this means
      sending a response containing a new cookie (§2.6)

    - [ ] [RFC7296-2.6.1-1] [MUST NOT] Implementations SHOULD support this shorter exchange,
      but MUST NOT fail if other implementations do not support this shorter exchange (§2.6.1)

Each row is ONE physical line in the file; the wrapping above is for this document only,
because `parse_checklist_line` reads one row per line.

`1.2-6` and `2.6-5` are deliberately LONGER than Appendix A's versions (sections 1.2 and
1.5). Both restorations return clauses that carry the security rationale, and `2.6-5`'s
restored clause is what stops a future reader hardening the fail-open. Correct Appendix A to
match in the same commit.

After the rows land, run `make ze-rfc-index-update` and commit `ai/RFC-REQUIREMENTS.md` in the SAME
commit: the ledger records each tagged test's `file:line`, and both verify modes of
`ze-rfc-check` fail on a stale ledger (`ai/rules/testing.md`, RFC-Tagged Tests). Add the six
rows to `docs/features/rfc-status.md`'s RFC 7296 entry as well -- `check_status_completeness`
gates the public disclosure, and `check_gap_count_agreement` gates the Remaining count.

---

## 12. Verification

| Gate | Command | Why |
|------|---------|-----|
| Unit, engine package | `make ze-unit-bgp-test` does not cover it; use `go test -race -count=20 ./internal/component/ike/...` | R-WP4-7. The `-count=20` is the reactor-style stress, applied to the package that actually changed |
| Functional | `make ze-functional-test` | the eight `test/ipsec/*.ci`, plus the new one |
| Interop, before AND after | `make ze-interop-ipsec-test` | the eleven existing scenarios, four of which are responder-side. Section 8 |
| One scenario while iterating | `make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=12-invalid-ke-retry` | `mk/test-integration.mk` |
| Ledger | `make ze-rfc-index-update` then `make ze-rfc-check` | section 11 |
| Full gate | `make ze-precommit-verify` | `ai/rules/git-safety.md` |

Draft every new `.ci` in `test/draft/ipsec/` and promote it only when green, per
`ai/rules/testing.md` ("Draft a Functional Test Before It Is Live").

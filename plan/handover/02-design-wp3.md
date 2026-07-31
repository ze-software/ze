<!-- ste: ignore-file preserved verbatim from the design pass. It quotes RFC 7296 at length and carries producer citations for error notification emission. An edit for prose style risks altering a technical claim the next session relies on, and the document is a working record rather than published prose. -->

# WP-3 design: error notification emission and consumption

Work package 8 of `plan/spec-rfcgate-1b-rfc7296-pilot.md` ("WP-3"), 13 rows.

Read-only research pass. No tracked file was modified. Every `file:line` below was
read in this session at the producing function, per `ai/rules/no-fabrication.md`.

---

## 0. What changed since the brief was written

The brief states that "there is no protected error-notification sender at all" and
that "the only protected error notify anywhere is `sendAuthFailed`". **That is no
longer true of this tree.** A second protected sender landed with the in-flight work:

`respondIKERekey` builds an SK-encrypted CREATE_CHILD_SA response carrying
INVALID_KE_PAYLOAD (`internal/component/ike/engine/rekey.go:511-525`), and
`handleCreateChildSAOwned` sends it (`internal/component/ike/engine/inbound.go:230-240`).

This matters twice. It gives WP-3 a correct existing shape to generalise rather than
invent. And it makes the gap sharper, not softer: the two sibling responder functions
now disagree about the same class of refusal. `respondIKERekey` answers a KE-group
mismatch with a notify; `respondChildRekey` answers a proposal mismatch with silence
(`rekey.go:172-175`). That is a sibling call-site divergence
(`ai/rules/before-writing-code.md`), and WP-3 closes it.

Everything else in the brief holds. The live violation is confirmed below.

---

## 1. Row by row

Verdict key: **MET** (needs a tagged pair only), **MET-VACUOUS** (holds because the
code path does not exist; WP-3 makes it a real guard), **HOLE** (met on every path
but one), **ABSENT** (not implemented).

### `RFC7296-2.5-10` [MUST] -- ABSENT

> "If the critical flag is set and the payload type is unrecognized, the message MUST
> be rejected and the response to the IKE request containing that payload MUST
> include a Notify payload UNSUPPORTED_CRITICAL_PAYLOAD, indicating an unsupported
> critical payload was included. **In that Notify payload, the Notification Data
> contains the one-octet payload type.**"
> `rfc/full/rfc7296.txt:1704-1709`

Ze today. The two parsers detect the condition and both return a bare sentinel:
`Message.ReadFrom` at `internal/component/ike/wire/message.go:126-128`, and
`ParsePayloadChain` at `internal/component/ike/wire/chain.go:37-39`. Both return
`ErrUnsupportedCrit` (`internal/component/ike/wire/payload.go:22`). No caller emits a
notify: `internal/component/ike/engine/inbound.go:38-41` and
`internal/component/ike/engine/fsm.go:337-340` both log at Debug and return.
`NotifyUnsupportedCriticalPayload` (`internal/component/ike/wire/payload_notify.go:9`)
has zero non-test referents.

Second defect, and it is structural. **The sentinel carries no payload type**, so the
Notification Data clause is unreachable without a codec change. The bolded sentence is
dropped from the Appendix A row text and must be restored before the row lands.

### `RFC7296-2.21.2-1` [MUST] -- ABSENT (two distinct defects)

> "request messages that contain an unsupported critical payload, or where the whole
> message is malformed (rather than just bad payload contents), MUST be rejected in
> their entirety, and MUST only lead to an UNSUPPORTED_CRITICAL_PAYLOAD or
> INVALID_SYNTAX Notification sent as a response."
> `rfc/full/rfc7296.txt:3291-3295`

Ze today. "Rejected in their entirety" holds: a parse error drops the whole message.
"MUST only lead to a Notification sent as a response" does not: no notification is
sent at any of the three parse-error sites (`inbound.go:38-41`, `fsm.go:337-340`,
`fsm.go:559-563`).

The second defect is upstream of the first. `ParsePayloadChain` does not report a
truncated inner chain as an error at all. It breaks out of the loop and returns the
payloads it already holds, at `internal/component/ike/wire/chain.go:19` and
`internal/component/ike/wire/chain.go:30`. A malformed message therefore presents as a
message missing a payload, and a caller takes a different branch entirely (for example
`fsm.go:615-618`, "AUTH response missing AUTH payload"). **Making `ParsePayloadChain`
return an error on truncation is a prerequisite for this row and for `3.10.1-3`, not
an optional extra.**

### `RFC7296-2.21.2-2` [MUST NOT] -- MET

> "a responder may include all the payloads associated with authentication (IDr, CERT,
> and AUTH) while sending error notifications for the piggybacked exchanges
> (FAILED_CP_REQUIRED, NO_PROPOSAL_CHOSEN, and so on), and the initiator MUST NOT fail
> the authentication because of this."
> `rfc/full/rfc7296.txt:3311-3314`

Ze today. `handleAuthResponse`'s payload walk aborts on exactly one notify type,
`NotifyAuthenticationFailed` (`internal/component/ike/engine/fsm.go:576-582`). Every
other notify type falls through the `switch` untouched. AUTH is then verified
(`fsm.go:620-624`) and the SA is established (`fsm.go:643`). The accepted-offer
consistency check is guarded by `childOffer != nil` (`fsm.go:630-637`), so a response
that carries an error notify instead of SAr2 skips it rather than failing on it.

**Needs a tagged pair, no code.**

One adjacent clause to check during implementation, and it is NOT an owed row. RFC
`:3298-3301` says "This failure does not automatically cause the IKE SA to be
deleted". It is indicative, not MUST-level, so D7 does not bite. But Ze's initiator
reaches `createFirstChildSA` from `maintainSA` (`established.go:48-54`) with
`sa.ChildOutboundSPI == 0`, and a failure there returns an error that ends the
session. Verify what actually happens; do not assume the IKE SA survives.

### `RFC7296-2.21.2-3` [MUST NOT] -- MET BY ABSENCE

> "Extension documents may define new error notifications with these semantics, but
> MUST NOT use them unless the peer has been shown to understand them, such as by
> using the Vendor ID payload."
> `rfc/full/rfc7296.txt:3322-3324`

Ze today. Ze defines no extension error notification. Every constant in
`internal/component/ike/wire/payload_notify.go:9-39` comes from RFC 7296 or a cited
RFC (7383 at `:38`, 7427 at `:39`), and none is used with the semantics the sentence
governs. The spec already records this row as one of seven with no site at all
(`plan/spec-rfcgate-1b-rfc7296-pilot.md:1595`, "No extension-notify surface").

This is the expiring-negative shape the spec records at
`plan/spec-rfcgate-1b-rfc7296-pilot.md:1843-1863`. The tag must say the argument is an
absence and that it expires when an extension-notify surface arrives. Do not dress it
up as something stronger.

### `RFC7296-2.21.3-1` [MUST] -- ABSENT. This is the live violation.

> "After the IKE SA is authenticated, all requests having errors MUST result in a
> response notifying the other end of the error."
> `rfc/full/rfc7296.txt:3328-3329`

Four producers on the authenticated path answer with silence:

| Site | What happens | When it is reached |
|---|---|---|
| `internal/component/ike/engine/inbound.go:191-195` | `respondChildRekey` returns an error, the caller logs at Warn and returns `ownedOutcome{}` | the peer's CREATE_CHILD_SA rekey offer matches no configured ESP proposal (`rekey.go:172-175`, `crypto.ErrNoProposalChosen`), or the request lacks Ni or the ESP SPI (`rekey.go:165-167`) |
| `internal/component/ike/engine/inbound.go:231-234` | `respondIKERekey` returns an error, same shape | every `respondIKERekey` error other than the KE-group mismatch, which does answer (`rekey.go:511-525`) |
| `internal/component/ike/engine/inbound.go:38-41` | outer parse failure, log at Debug, return | a malformed established-SA message |
| `internal/component/ike/engine/inbound.go:248` | "peer new-child request unsupported, ignoring" | a CREATE_CHILD_SA that is neither a Child rekey nor an IKE rekey, that is, a request for a NEW Child SA |

The first row is the one the review traced. A peer whose `esp-group` drifts by one
algorithm rekeys its Child SA, gets nothing back, spends its single request window
(`sa.reserveRequestWindow`, `msgid.go:64-72`) on retransmissions, and closes the IKE
SA. RFC 7296 designed a one-payload response for that condition
(`rfc/full/rfc7296.txt:1974-1976`, "The responder MUST accept a single proposal or
reject them all and return an error. The error is given in a notification of type
NO_PROPOSAL_CHOSEN.").

The fourth row is a `2.21.3-1` violation whichever notify is correct for it. The
notify the RFC names for that case is NO_ADDITIONAL_SAS, which is WP-12's `4-1`. WP-3
owns the SILENCE, not the choice of constant. Use NO_PROPOSAL_CHOSEN, which
`rfc/full/rfc7296.txt:5680-5681` explicitly blesses as the "generic Child SA error
when Child SA cannot be created for some other reason", and let WP-12 refine it.

A fifth site sits on the boundary and belongs here. `handleAuthRequest` at
`internal/component/ike/engine/responder.go:424-429`: after `verifyRemoteAuth` has
already passed at `:417`, a `buildAuthResponse` failure logs and sets `StateDead` with
no response at all. `selectResponderESP` (`responder.go:443-466`) returns
`crypto.ErrNoProposalChosen` on that path. Section 2.21.2 governs this one rather than
2.21.3, and it says the opposite of what Ze does: send IDr, CERT and AUTH together
with the error notify, and do not delete the IKE SA.

### `RFC7296-2.21.4-1` [MUST NOT] -- MET-VACUOUS

> "If the message is marked as a response, the node can audit the suspicious event but
> MUST NOT respond."
> `rfc/full/rfc7296.txt:3355-3356`

Ze today. An out-of-SA message reaches the `sa == nil` branch of either dispatch loop:
`internal/component/ike/engine/register.go:665-670` (port 500) and
`internal/component/ike/engine/register.go:490-495` (port 4500). Both log at Debug and
`continue`. Nothing is ever sent, so the row holds because the sender does not exist.

WP-3 adds that sender. **This row is the guard that keeps it from answering a
response**, and it moves from vacuously held to actively enforced.

### `RFC7296-2.21.4-2` [MUST] -- ABSENT (vacuous)

> "If a response is sent, the response MUST be sent to the IP address and port from
> where it came with the same IKE SPIs and the Message ID copied."
> `rfc/full/rfc7296.txt:3358-3368`

Section 1.5 gives the same message a fuller construction rule, and it resolves every
ambiguity Section 2.21.4 leaves open:

> "the message is always sent without cryptographic protection (outside of an IKE SA),
> and includes either an INVALID_IKE_SPI or an INVALID_MAJOR_VERSION notification
> (**with no notification data**). The message is a response message, and thus it is
> sent to the IP address and port from whence it came with the same IKE SPIs and the
> **Message ID and Exchange Type are copied** from the request. **The Response flag is
> set to 1**, and the version flags are set in the normal fashion."
> `rfc/full/rfc7296.txt:1073-1080`

Ze today. No such response exists. The nearest existing shape is `sendSAInitNotify`
(`internal/component/ike/engine/responder.go:329-355`) and it **cannot be reused**: it
hardcodes `MessageID: 0` (`responder.go:341`) and writes `sa.ResponderSPI`, which is a
freshly generated value (`responder.go:26-29`), not the SPI from the request. Both are
correct for an IKE_SA_INIT response inside a known SA and both are wrong here.

### `RFC7296-2.21.4-3` [MUST NOT] -- ABSENT (vacuous)

> "The response MUST NOT be cryptographically protected"
> `rfc/full/rfc7296.txt:3368-3369`

Vacuous today. Made structural below: the emitter takes no `*SA`, so it holds no keys
and cannot protect anything.

### `RFC7296-2.21.4-4` [MUST] -- ABSENT

> "and MUST contain an INVALID_IKE_SPI Notify payload"
> `rfc/full/rfc7296.txt:3369`

Ze today. `NotifyInvalidIKESPI` (`internal/component/ike/wire/payload_notify.go:10`)
has zero non-test referents.

### `RFC7296-2.21.4-5` [MUST NOT] -- HOLE. Met on every path but one.

> "A peer receiving such an unprotected Notify payload MUST NOT respond"
> `rfc/full/rfc7296.txt:3374`

Ze today, the part that holds. On an established SA every processing branch of
`handleOwnedInbound` funnels through `decryptAndParse`
(`internal/component/ike/engine/inbound.go:73-77`), which requires an SK payload
(`inbound.go:112-114`) and a passing integrity check. An unprotected notify therefore
reaches no responder, whether it is marked as a request or as a response. The
`inboundInvalid` INFORMATIONAL branch (`inbound.go:57-65`) likewise gates on a
successful decrypt. Before establishment, `handleResponderInbound` drops any message
with the R bit set (`responder.go:52-55`).

Ze today, the hole. `classifyInbound` runs BEFORE authentication, and its own doc
comment says so (`internal/component/ike/engine/msgid.go:120-122`). A datagram whose
Message ID equals `sa.lastResponseID`, with the R bit clear, classifies as
`inboundRetransmit`, and the branch at `internal/component/ike/engine/inbound.go:45-50`
replays `sa.lastResponse` **with no decrypt at all**. An unprotected notify shaped that
way draws a response. Both SPIs and the Message ID travel in the clear in every IKE
header, and Section 2.21.4 deliberately copies them into the unauthenticated response,
so an attacker who has seen one packet has everything the forgery needs.

**The blast radius today is bounded, and the reason is worth stating precisely.**
`sendRaw` sends to `sa.remoteUDPAddr()` (`internal/component/ike/engine/established.go:312-323`),
which resolves `sa.PeerCfg.RemoteAddress`, the CONFIGURED peer
(`internal/component/ike/engine/sa.go:193-199`). A spoofed source address cannot
redirect the reply, so this is a nuisance against the legitimate peer rather than a
general amplifier at a chosen target.

**It becomes a general spoofable amplifier the moment WP-8 lands.** WP-8 (spec phase
item 11, rows `2.11-2`, `2.11-3`, `2.23-4`) changes established-path replies to go to
the OBSERVED source address and port. **The spec's ordering of WP-3 before WP-8 is
therefore load-bearing and must not be swapped.**

### `RFC7296-2.21.4-6` [MUST NOT] -- MET

> "and MUST NOT change the state of any existing SAs"
> `rfc/full/rfc7296.txt:3374-3375`

Ze today. Same structural argument as `-5`, and it survives the hole that `-5` does
not. Every state mutation on an established SA sits behind `decryptAndParse`
(`inbound.go:73-77`). The one pre-authentication branch, `inboundRetransmit`
(`inbound.go:45-50`), reads `sa.lastResponseSet` and `sa.lastResponse` and calls
`sendRaw`; it writes nothing and returns `ownedOutcome{}`, so `out.peerAlive` stays
false and even the DPD liveness timer is not reset (contrast `inbound.go:97`).

`-5` and `-6` are usually assumed to stand or fall together. Here they do not. State
that in the tag, or the next reader will treat one proof as covering both.

### `RFC7296-2.21.4-7` [MUST NOT] -- MET. Different scope from `-6`.

> "The recipient MUST NOT change the state of any SAs as a result, but may wish to
> audit the event to aid in diagnosing malfunctions."
> `rfc/full/rfc7296.txt:3392-3393`

**This row does not govern the same message as `-6`.** It closes the last paragraph of
Section 2.21.4 (`rfc/full/rfc7296.txt:3389-3393`), which is about "a suspicious message
from an IP address (and port, if NAT traversal is used) with which it has an IKE SA"
answered by "an IKE Notify payload in an IKE INFORMATIONAL exchange over that SA". So
`-7` binds the recipient of a **protected** INFORMATIONAL notify. `-6` binds the
recipient of the **unprotected** INVALID_IKE_SPI.

Appendix A's two row texts are nearly identical and a reader will conflate them. This
is exactly the `RFC7296-1.4.1-4` trap the spec records at
`plan/spec-rfcgate-1b-rfc7296-pilot.md:1816-1841`: two rows govern one observable, and
only the section scope separates them. A test that asserts "no state changed" proves
neither row until it establishes which message it is holding.

Ze today. `handleInformationalOwned` walks the decrypted inner chain and acts on
`*wire.PayloadDelete` alone (`internal/component/ike/engine/inbound.go:299-317`). A
`*wire.PayloadNotify` in a protected INFORMATIONAL request changes nothing. It does
draw a response (`inbound.go:319-328`), which is correct: `-7` forbids a state change,
not a response, and `RFC7296-1.4-4` requires the response.

### `RFC7296-3.10.1-3` [MUST] -- ABSENT

> "To avoid leaking information to someone probing a node, this status MUST be sent in
> response to any error not covered by one of the other status types."
> `rfc/full/rfc7296.txt:5663-5665`

**"this status" is INVALID_SYNTAX.** The sentence sits inside the INVALID_SYNTAX table
entry (`rfc/full/rfc7296.txt:5649-5667`). The Appendix A row drops the antecedent and
is unreadable on its own. Re-author it before it lands, the same treatment the spec
applied to `3.3.6-4` and `3.3.6-5` at
`plan/spec-rfcgate-1b-rfc7296-pilot.md:1910-1913`.

**The same table entry carries a precondition that constrains the whole package:**

> "To avoid a DoS attack using forged messages, this status may only be returned for
> and in an encrypted packet if the Message ID and cryptographic checksum were valid."
> `rfc/full/rfc7296.txt:5652-5655`

Ze today. `NotifyInvalidSyntax` (`internal/component/ike/wire/payload_notify.go:12`)
has zero non-test referents.

### Tally

| Verdict | Count | Rows |
|---|---|---|
| MET, needs a tagged pair only | 4 | `2.21.2-2`, `2.21.2-3`, `2.21.4-6`, `2.21.4-7` |
| MET-VACUOUS, becomes a real guard | 1 | `2.21.4-1` |
| HOLE, one path violates | 1 | `2.21.4-5` |
| ABSENT | 7 | `2.5-10`, `2.21.2-1`, `2.21.3-1`, `2.21.4-2`, `2.21.4-3`, `2.21.4-4`, `3.10.1-3` |

---

## 2. The protected error-notify sender

### Is `engine/notify_error.go` the right home?

Yes for the file. **No for a single function.** Two senders live here and their
constraints are opposite:

| | Protected | Unprotected |
|---|---|---|
| Needs `*SA` and `SKKeys` | yes | **must not have them** |
| Destination | `sa.remoteUDPAddr()` via `sendRaw` | `pkt.RemoteAddr`, verbatim |
| Message ID | the request's, from a decrypted, window-checked message | the request's, copied from a header nobody authenticated |
| Cached for retransmission | yes, `cacheResponse` | never, there is no SA to cache on |
| Rate limited | no, an authenticated peer paid for it | yes, mandatory |

Put both in `internal/component/ike/engine/notify_error.go`, with a file comment
stating why they must never be merged. A merged helper that takes an optional `*SA`
is the shape that eventually protects the unprotected response or leaks keys into the
out-of-SA path.

### Signature: build and return, do not send

Follow `respondIKERekey` (`rekey.go:511-525`), not `sendAuthFailed`
(`responder.go:692-704`). The build-and-return shape is right because `cacheResponse`
advances `sa.ExpectedMsgID` (`msgid.go:145-151`) and is required for the Section 2.1
retransmission contract, and because only the caller knows the exchange type. It also
keeps the owner loop the single writer of SA state (`inbound.go:30-35`).

```
// buildErrorNotifyResponse builds the SK-encrypted response that answers a request
// Ze refused. RFC 7296 Section 2.21.3. It never sends: the caller MUST cacheResponse
// then sendRaw, so a retransmitted request draws the same bytes (Section 2.1).
func buildErrorNotifyResponse(
    sa *SA, msgID uint32, exchange uint8, notifyType uint16, data []byte,
) ([]byte, error)
```

Body: one `wire.PayloadNotify`, `SPISize: 0`, into
`buildEncryptedMessageEx(sa, inner, msgID, exchange, initiatorFlag(sa)|wire.FlagResponse)`.
`initiatorFlag(sa)` rather than a literal, because `RFC7296-3.1-12` is a WP-2 row and
`dpd.go` is already the one site that got it wrong.

The three-line caller tail repeats at five sites, which is the second use case that
justifies the wrapper (`ai/rules/design-principles.md`, "Abstract at the second use
case"):

```
func (ps *PeerSession) respondError(
    sa *SA, msgID uint32, exchange uint8, notifyType uint16, data []byte,
    tr *transport.UDPTransport, log *slog.Logger,
)
```

### Call sites

| Site | Notify | Basis |
|---|---|---|
| `inbound.go:191-195` (`respondChildRekey` error) | NO_PROPOSAL_CHOSEN when `errors.Is(err, crypto.ErrNoProposalChosen)`, else INVALID_SYNTAX | `rfc7296.txt:1974-1976`; `:5663-5665` |
| `inbound.go:231-234` (`respondIKERekey` error) | same split | same |
| `inbound.go:248` (new-child request unsupported) | NO_PROPOSAL_CHOSEN as the generic Child SA error | `rfc7296.txt:5680-5681` |
| `responder.go:424-429` (`buildAuthResponse` error after AUTH verified) | NO_PROPOSAL_CHOSEN, **piggybacked with IDr, CERT and AUTH**, and the IKE SA is NOT set to `StateDead` | `rfc7296.txt:3298-3315` |
| the INNER parse failure inside `decryptAndParse` (`inbound.go:118`) | INVALID_SYNTAX | `rfc7296.txt:5652-5665` |

### The single most important design point in this package

**An OUTER parse failure MUST NOT draw INVALID_SYNTAX. It must stay a silent drop.**

`msg.ReadFrom` at `inbound.go:38` runs on the outer message, before decryption. A
failure there means Ze never located the SK payload, so the Message ID and the
cryptographic checksum were never validated. `rfc/full/rfc7296.txt:5652-5655` says
INVALID_SYNTAX "may only be returned for and in an encrypted packet if the Message ID
and cryptographic checksum were valid". Emitting there turns a 28-byte forgery into a
guaranteed response and creates precisely the DoS the sentence names.

INVALID_SYNTAX belongs to the INNER parse failure, after `decryptSKPayload` has already
succeeded: inside `decryptAndParse` (`inbound.go:115-118`), at `fsm.go:559-563`, and at
`fsm.go:719`. Get this backwards and the package ships an amplifier.

### Two wire-package prerequisites

**P1. `ParsePayloadChain` must report truncation.** The two silent `break`s at
`internal/component/ike/wire/chain.go:19` and `:30` must return `ErrTruncated`, matching
`Message.ReadFrom` (`message.go:113-114`). Until then `2.21.2-1` and `3.10.1-3` have no
site to fire from. Check every caller for a dependence on the lenient behaviour before
changing it.

**P2. `ErrUnsupportedCrit` must carry the payload type.** `2.5-10` requires the
one-octet type in the Notification Data (`rfc7296.txt:1708-1709`) and the sentinel
discards it. Replace it with a typed error that keeps `errors.Is` working:

```
type UnsupportedCritError struct{ PayloadType uint8 }
func (e *UnsupportedCritError) Error() string
func (e *UnsupportedCritError) Is(target error) bool  // true for ErrUnsupportedCrit
```

**This is safe, and it is verified rather than assumed.** Every existing comparison in
the tree uses `errors.Is`, never `==`: `wire/rfc7296_innerchain_test.go:78` and `:207`,
`wire/payload_test.go:82`, `wire/rfc7296_test.go:319`, `:363` and `:448`. So the two
tagged tests for `RFC7296-2.5-9` stay green with no behaviour change, and the
`rfc-tagged-test` hook is not tripped.

---

## 3. The unprotected out-of-SA path

Different constraints, stricter, and a separate function.

### Location

The `sa == nil` branch of each dispatch loop, AFTER `tryResponderSAInit` declines:
`internal/component/ike/engine/register.go:665-670` (port 500) and
`internal/component/ike/engine/register.go:490-495` (port 4500).

### Signature: the guarantee is what is absent from it

```
func sendInvalidIKESPI(
    tr *transport.UDPTransport,
    pkt transport.Packet,
    hdr wire.Header,
    limiter *outboundNotifyLimiter,
    log *slog.Logger,
)
```

No `*SA`. No `*PeerSession`. No keys reachable from any argument. `2.21.4-3` becomes a
property of the signature rather than of a branch, which is the strongest available
form under `ai/rules/fail-closed-guards.md`.

It must NOT parse the payload chain. It needs the 28-byte header alone, and
`dispatchInbound` has already validated length and major version
(`register.go:641-647`). Parsing attacker bytes to decide whether to answer them is a
strictly larger attack surface for no benefit.

### Construction

From `rfc/full/rfc7296.txt:1073-1080` and `:3358-3369`:

| Field | Value | Locator |
|---|---|---|
| InitiatorSPI, ResponderSPI | copied from the request | `:3367` "the same IKE SPIs" |
| ExchangeType | copied from the request | `:1078-1079` |
| MessageID | copied from the request | `:3367-3368` |
| Flags | `wire.FlagResponse` only. I bit clear, V bit clear | `:1079-1080`; `RFC7296-3.1-7`, `-3.1-11` |
| MajorVersion / MinorVersion | 2 / 0 | `RFC7296-3.1-5`, `-3.1-6` |
| Payload | one `wire.PayloadNotify{NotifyMsgType: wire.NotifyInvalidIKESPI}`, `SPISize: 0`, `NotificationData: nil` | `:3369`; `:1075-1076` "with no notification data" |
| Destination | `pkt.RemoteAddr`, verbatim | `:3358-3367` |
| Socket | `tr`, the transport the packet arrived on | `tr.Send` uses `WriteToUDP` (`internal/component/ike/transport/udp.go:71`), so **both sockets are already send-capable**; no WP-8 dependency |
| NAT-T | on the 4500 path, re-add the marker with `transport.AddNonESPMarker` (`internal/component/ike/transport/nat.go:58`) | RFC 3948 Section 2.2 |

The notify codec already writes a zero Protocol ID beside an empty SPI
(`internal/component/ike/wire/payload_notify.go:65-73`), so `RFC7296-3.10-3` and
`-3.10-5` are satisfied by the existing encoder.

### Four preconditions, every one denying by sending nothing

1. `hdr.Flags&wire.FlagResponse != 0` -> send nothing. **This is `2.21.4-1`.**
2. `hdr.ExchangeType == wire.ExchangeIKESAInit` -> send nothing. `rfc7296.txt:3353`
   excludes "a request to start an IKE SA". **Check the exchange type explicitly. Do
   not infer it from `tryResponderSAInit` returning false**: that function also returns
   false for an unconfigured source (`register.go:595-597`) and for a non-zero
   responder SPI (`register.go:591-592`), and both of those are still IKE_SA_INIT
   requests.
3. The outbound limiter denies -> send nothing. `rfc7296.txt:3349-3350`.
4. `pkt.RemoteAddr == nil` -> send nothing.

### A new outbound limiter, not the existing one

`inboundRateLimiter` (`internal/component/ike/engine/register.go:409-445`, constructed
at `:452` and `:638` with 100/s and burst 200) gates PROCESSING and is shared with all
legitimate traffic. Three reasons for a separate one:

- `rfc7296.txt:3349-3350` is specifically about SENDING.
- A flood of forged out-of-SA packets would otherwise consume the budget that keeps
  real sessions alive.
- `RFC7296-2.3-6` (WP-1) needs the same primitive for INVALID_MESSAGE_ID, and the spec
  records that none exists to extend
  (`plan/spec-rfcgate-1b-rfc7296-pilot.md:1592`).

Budget: 1 per second sustained, burst 5, keyed per source IP if the map is bounded,
else global. That is two orders of magnitude tighter than the inbound limiter, because
every packet it gates is by definition unauthenticated and the legitimate rate is near
zero: a real crashed peer sends one request, not a hundred.

Metrics, already required by the spec's Integration Checklist
(`plan/spec-rfcgate-1b-rfc7296-pilot.md:565`): register
`ike_error_notify_sent_total{type,protected}` and
`ike_error_notify_suppressed_total{reason}` in the owning package.

### One adjacent consumer to note

The third case of `rfc/full/rfc7296.txt:1039-1040` and `:1073-1076` is
INVALID_MAJOR_VERSION for a request with a higher major version. Ze drops that silently
at `register.go:643-645` and `register.go:465-467`. It is not one of the 13 rows, but it
is the same emitter with a different constant. Leave the seam open for it.

---

## 4. Loop prevention

**The invariant, stated once: Ze never answers a message that is not cryptographically
protected, with the single exception of the unprotected INVALID_IKE_SPI, and that one
is never sent in reply to a response.**

Three checks, each placed at the producer that knows.

| # | Check | Where it goes | Row |
|---|---|---|---|
| L1 | R bit set, or exchange is IKE_SA_INIT, or the limiter denies -> emit nothing | inside `sendInvalidIKESPI`, before any build | `2.21.4-1` |
| L2 | An unprotected datagram never draws the cached response | the `inboundRetransmit` branch, `internal/component/ike/engine/inbound.go:45-50` | `2.21.4-5` |
| L3 | An error notify is only ever sent AS A RESPONSE, never as a new INFORMATIONAL request | the contract of `buildErrorNotifyResponse`: it takes `msgID` and always sets `wire.FlagResponse` | `rfc7296.txt:3334-3336` |

### L2's shape is constrained, and the obvious stronger guard is forbidden

Replay the cache only when the outer message carries a `*wire.PayloadSK`. That test is
structural, needs no keys, and costs one pass over `msg.Payloads`.

- Every genuine retransmission of a post-IKE_AUTH request carries SK by construction
  (`RFC7296-1.4-5`: INFORMATIONAL and CREATE_CHILD_SA are always protected).
- **It leaves the existing tagged test green, unmodified.**
  `TestRtxResponderReplaysCachedResponseOnlyForDuplicate`
  (`internal/component/ike/engine/rfc7296_retransmit_test.go:167`) feeds a real
  `buildEncryptedMessageEx` Delete, which carries SK. That test carries four tags
  (`RFC7296-2.1-3` and `-2.1-4`, both polarities) and the `rfc-tagged-test` hook blocks
  any behaviour change to it without the user's explicit approval.
- **A full decrypt before replay is forbidden, not merely unattractive.** The same test
  asserts at `rfc7296_retransmit_test.go:163` that "the engine never decrypts it", and
  the cache exists so that it does not have to. A reviewer will propose the full
  decrypt; this paragraph is why the answer is no.

### The fixed-point argument

The strongest available proof that WP-3 creates no loop is that the emitter's own
output is not an input the emitter answers. Take the exact bytes `sendInvalidIKESPI`
produces, feed them back into the same out-of-SA branch, and assert nothing is
emitted. It is true by L1, since the output has the R bit set. It is one step, it is
deterministic, and it needs no timing.

The two-daemon `.ci` then proves the same property through the real socket path,
where a mistake would actually run away.

---

## 5. Test plan

### Files

| File | Holds |
|---|---|
| `internal/component/ike/engine/notify_error_test.go` (new) | the two emitters, the four preconditions, the fixed point |
| `internal/component/ike/engine/rfc7296_errornotify_test.go` (new) | the Section 2.21.2 and 2.21.3 rows against the real handlers |
| `internal/component/ike/wire/rfc7296_critpayload_test.go` (new) | `2.5-10`'s Notification Data, the typed error, `ParsePayloadChain` truncation |
| `test/ipsec/ipsec-error-notify-no-loop.ci` (new) | two Ze instances, no loop |
| `test/ipsec/ipsec-child-rekey-no-proposal.ci` (new) | the review's traced symptom, end to end |
| `test/ipsec-interop/scenarios/19-error-notifications/` (new) | strongSwan |

### The seams already exist. Use them.

- `rtxPeerLink(t)` (`internal/component/ike/engine/rfc7296_retransmit_test.go:30-56`)
  gives a loopback peer transport plus the socket Ze sends from, and points
  `SA.remoteUDPAddr` at the peer through the `ikeTestPortFn` seam.
- **`rtxExpectSilence(t, peerTr, myTr, remote, what)`
  (`rfc7296_retransmit_test.go:70-81`) is a sound silence assertion**: it sends a
  sentinel and requires the sentinel to be the NEXT datagram the peer reads. It is not
  a timeout race. Every negative below that asserts "nothing was sent" uses it.
- `ps.handleOwnedInbound(sa, transport.Packet{Data: ...}, myTr, nil, log)` drives the
  owned path directly, as at `rfc7296_window_test.go:145` and
  `rfc7296_retransmit_test.go:181`.
- `establishPSK(t)` returns an established initiator, responder and session.

### Which rows need a `.ci`

`test/ipsec/` earns the `functional/verify` tier: it is in `all_suites`
(`mk/test-functional.mk:192`) AND has a `run_suite ipsec` line
(`mk/test-functional.mk:217`). Both are required, and the spec records what happens
when only the first is present (`plan/spec-rfcgate-1b-rfc7296-pilot.md:1463-1479`).
`ipsec-show-sa.ci` already runs two unprivileged Ze instances over loopback with the
noop dataplane, so the harness for a two-daemon test exists.

| Row | `.ci` | Why |
|---|---|---|
| `2.21.4-5` | **yes** | the worst blast radius, and the only failure mode that runs away between two live daemons. Unit tests prove one step; only two daemons prove there is no second |
| `2.21.3-1` | **yes** | it is the user-visible symptom the review traced. `ai/rules/functional-test-gate.md` requires the end-to-end proof for a user-facing behaviour change |
| the other 11 | unit only | wire construction and branch selection, with no user entry point of their own |

### Per row: positive, negative, and the mutation that kills each

**`2.5-10`**
- Positive: parse a message carrying an unknown critical payload of type 199; assert the
  response carries `NotifyUnsupportedCriticalPayload` with
  `NotificationData == []byte{199}`. Mutation: return a nil `NotificationData`.
- Negative: the same payload with the critical bit CLEAR draws no notify and is demoted
  to `PayloadRaw` (`wire/message.go:130`). Mutation: ignore the critical bit.

**`2.21.2-1`**
- Positive: a message whose inner chain is truncated mid-payload draws INVALID_SYNTAX.
  Mutation: restore the `break` at `wire/chain.go:30`, so the error never arrives.
- Negative, and it is the valuable half: a well-formed message with a bad payload
  CONTENT (an unknown transform inside SA, which `isItemRejected` tolerates at
  `wire/message.go:117-121`) draws NO INVALID_SYNTAX. Mutation: treat `isItemRejected`
  as fatal. **This negative is what stops the row from over-rejecting, and
  over-rejecting is the failure mode that breaks the strongSwan lab.**

**`2.21.2-2`**
- Positive: feed `handleAuthResponse` an IKE_AUTH response carrying IDr, AUTH and
  `NotifyNoProposalChosen`, with no SAr2; assert `sa.State == StateEstablished`.
  Mutation: add `NotifyNoProposalChosen` to the abort switch at `fsm.go:576-582`.
- Negative: the same response carrying `NotifyAuthenticationFailed` instead DOES kill
  the SA, so the positive is not "all notifies are ignored". Mutation: remove the
  AUTHENTICATION_FAILED arm.

**`2.21.2-3`**
- Positive: assert every `NotifyMsgType` Ze can transmit is drawn from the
  RFC-registered set. Derive the sent-set from the emitter table, never a hardcoded
  list (`ai/rules/derive-not-hardcode.md`). Mutation: add a private-range notify, for
  example 40961, to a sender.
- Negative: **there is none that rests on a property the code has**, because the
  obligation governs a surface Ze does not have. Per
  `plan/spec-rfcgate-1b-rfc7296-pilot.md:1843-1863` the tag must SAY the argument is an
  absence and that it expires when an extension-notify surface arrives, so the next
  reader knows why the test failed. Do not manufacture a stronger-looking negative.

**`2.21.3-1`**
- Positive: drive `handleCreateChildSAOwned` with a rekey request whose ESP offer
  matches nothing; assert a datagram is sent, that its decrypted inner chain carries
  `NotifyNoProposalChosen`, and that `sa.State` is still `StateEstablished`. Mutation:
  revert `inbound.go:191-195` to the bare `return ownedOutcome{}`.
- Negative: the same request with a matching offer draws a normal SAr2 response and no
  notify. Mutation: emit the notify unconditionally.
- `.ci`: `ipsec-child-rekey-no-proposal.ci`, modelled on `ipsec-child-rekey.ci`, with
  the responder's `esp-group` disjoint from the initiator's. Assert the initiator logs
  the received NO_PROPOSAL_CHOSEN and that `show vpn ipsec sa` still lists the peer as
  established.

**`2.21.4-1`**
- Positive: call the out-of-SA branch with the R bit set; `rtxExpectSilence` proves
  nothing was written. Mutation: drop the R-bit guard.
- Negative, and it is mandatory: the identical datagram with R CLEAR does draw
  INVALID_IKE_SPI. Without it the positive is the assertion-of-absence trap
  (`ai/rules/interop-and-goal-validation.md`): deleting the whole emitter yields the
  same silence.

**`2.21.4-2`**
- Positive: assert the emitted header's InitiatorSPI, ResponderSPI, MessageID and
  ExchangeType all equal the request's, and that the datagram arrived at the socket the
  request came from. **The fixture's request MUST use a non-zero Message ID, say 7.**
  `sendSAInitNotify` already hardcodes `MessageID: 0` (`responder.go:341`), so a
  copy-paste from it would pass a zero-Message-ID fixture. Mutation: set MessageID to
  0; separately, zero the ResponderSPI.
- Negative: a second request with a different SPI pair and Message ID draws a response
  carrying THOSE values, so the fields are copied rather than constant.

**`2.21.4-3`**
- Positive: parse the emitted datagram with `wire.Message.ReadFrom` and assert
  `Payloads` holds exactly one `*wire.PayloadNotify` and no `*wire.PayloadSK`.
- Negative: an established-SA error response built by `buildErrorNotifyResponse` for the
  same notify type DOES carry an SK payload, so the absence is a property of the
  unprotected emitter and not of the notify.
- The real guarantee is structural, and the tag must say so: `sendInvalidIKESPI` takes
  no `*SA`, so it holds no key material to protect anything with. A test can only
  confirm the output; the signature is what makes the property hold.

**`2.21.4-4`**
- Positive: the single payload is `NotifyInvalidIKESPI` with `SPISize == 0` and an empty
  `NotificationData`. Mutation: change the constant to `NotifyInvalidSyntax`.
- Negative: a request that trips a guard yields no payload at all, so the notify's
  presence is conditional on the emitter running.

**`2.21.4-5`** (two halves, both needed)
- Positive A, the fixed point: feed the emitter's own output back into the out-of-SA
  branch; `rtxExpectSilence`. Mutation: drop the R-bit guard, which makes it loop.
- Positive B: feed an established SA an unprotected notify at `sa.lastResponseID` with
  R clear; `rtxExpectSilence`. Mutation: remove the SK-presence guard from
  `inbound.go:45-50`.
- Negative: a genuine SK-carrying duplicate at the same Message ID DOES draw the cached
  response byte for byte. **That negative is already written**, as
  `TestRtxResponderReplaysCachedResponseOnlyForDuplicate`, and it must stay green
  unmodified.
- `.ci`: `ipsec-error-notify-no-loop.ci`. Two Ze instances over loopback on the
  `ipsec-show-sa.ci` pattern. Establish, then inject a forged unprotected notify at each
  daemon, then assert both SAs are still established and that neither daemon's packet
  count grew without bound. Count between two state transitions, never between two
  clock reads (`ai/rules/fix-dont-record.md`).

**`2.21.4-6`**
- Positive: snapshot `sa.State`, `sa.ExpectedMsgID`, `sa.NextMsgID`,
  `sa.requestOutstanding` and `ps.getChildSA()`; deliver the unprotected notify; assert
  every one is unchanged. Mutation: call `sa.answerAuthenticatedResponse` before the
  decrypt in `inbound.go`.
- Negative: the same notify delivered INSIDE a valid SK payload DOES reach
  `handleInformationalOwned` and does reset the DPD timer (`out.peerAlive = true`,
  `inbound.go:97`), which proves the state freeze is caused by the missing protection
  and not by the notify being inert everywhere.

**`2.21.4-7`**
- Positive: deliver a PROTECTED INFORMATIONAL request whose only inner payload is an
  error notify; assert a response is sent, per `RFC7296-1.4-4`, and that no SA or Child
  SA state changed. The tag must name the section scope, or it proves `-6` instead.
  Mutation: add a `case *wire.PayloadNotify:` arm to `inbound.go:300-317` that sets
  `sa.State = StateDead`.
- Negative: the same request carrying a `PayloadDelete` DOES change state, proving the
  notify's inertness is specific to notifies.

**`3.10.1-3`**
- Positive: an INNER-chain parse failure on an authenticated message draws
  INVALID_SYNTAX. Mutation: swap the constant for NO_PROPOSAL_CHOSEN.
- Negative, and it is the row's most valuable half: an OUTER parse failure
  (`inbound.go:38-41`, before any decryption) draws NOTHING. Mutation: move the
  INVALID_SYNTAX emission up to the outer parse error. This negative is the only thing
  that proves the `rfc7296.txt:5652-5655` DoS precondition is honoured.

### Interop scenario `19-error-notifications`

strongSwan proposes an `esp` suite Ze does not accept, on a CHILD REKEY rather than on
the initial exchange, since the initial exchange already fails cleanly. Assert that
strongSwan logs receipt of NO_PROPOSAL_CHOSEN and that the IKE SA stays up.

Vacuity check, required by `ai/rules/interop-and-goal-validation.md`: revert
`inbound.go:191-195` and confirm strongSwan instead times out and tears the SA down.
Record the mutation in the scenario.

---

## 6. Id allocation

`check_id_allocation` (`scripts/dev/rfc_requirements.py:477-510`) refuses a new id whose
ordinal is at or below its section's high-water mark, where the mark comes from the
committed HEAD summaries (`_git_baseline_ids`, `:1280`) and a section with NO mark at
all is skipped entirely (`:500-502`).

`rfc/short/rfc7296.md` is currently STAGED. **The baseline WP-3 must clear is therefore
the working-tree summary, which becomes HEAD when this commit lands**, not the
pre-commit HEAD. Marks computed against it:

| Section | Mark | Verdict for WP-3 |
|---|---|---|
| `RFC7296-2.5` | **13** (`RFC7296-2.5-13`, `rfc/short/rfc7296.md:532`) | **`2.5-10` IS REFUSED.** Needs -14 or higher |
| `RFC7296-2.21.2` | none | free at any ordinal |
| `RFC7296-2.21.3` | none | free at any ordinal |
| `RFC7296-2.21.4` | none | free at any ordinal |
| `RFC7296-3.10.1` | none | `3.10.1-3` accepted at -3, **but see the trap below** |

### §2.5: `2.5-10` must be renumbered, and four siblings compete for the same ordinals

Five Appendix A rows in §2.5 are unallocated, and **every one of them is below the mark
of 13**, so all five need renumbering: `2.5-3` and `2.5-4` (phase 3 and phase 2b),
`2.5-5` and `2.5-12` (WP-5), and `2.5-10` (WP-3). Whichever lands first takes -14.

Recommended: **land the five together, in one commit, in Appendix A ordinal order**,
which preserves document order and is stable under any later change.

| Appendix A id | New id | Owner |
|---|---|---|
| `2.5-3` | `2.5-14` | phase 3, owner ruling OR-D |
| `2.5-4` | `2.5-15` | phase 2b |
| `2.5-5` | `2.5-16` | WP-5 |
| **`2.5-10`** | **`2.5-17`** | **WP-3** |
| `2.5-12` | `2.5-18` | WP-5 |

If they cannot land atomically, the fallback rule is: recompute the mark at the moment
of landing, take mark+1, and correct Appendix A in the same commit. **WP-3 must not
hardcode -17 without re-running the computation.**

This is the hazard the spec already learned and wrote down at
`plan/spec-rfcgate-1b-rfc7296-pilot.md:1443-1445`: "land a section's rows together, or
land them in ascending ordinal order. A work-package grouping cuts across sections and
will trip this again." WP-3 is exactly such a package.

### §2.21.2, §2.21.3, §2.21.4: safe, and WP-3 owns all of them

Appendix A holds exactly 11 §2.21 rows and WP-3 owns all 11. No other package claims
one. So WP-3 lands `2.21.2-1..-3`, `2.21.3-1` and `2.21.4-1..-7` at their Appendix A
ordinals, in one commit, in ascending order. No renumbering.

### §3.10.1: the trap the brief warned about

The section has no mark, so `3.10.1-3` is accepted at -3 today. **Landing it alone sets
the mark to 3 and permanently blocks `3.10.1-1` and `3.10.1-2`,** which belong to other
phases: `3.10.1-1` is one of phase 3's five reclassified NOT IMPL rows
(`plan/spec-rfcgate-1b-rfc7296-pilot.md:1635`) and `3.10.1-2` is one of phase 3's eleven
now-provable rows (`:1625-1627`). Neither is in the summary.

- **Recommended: WP-3 lands `3.10.1-1`, `-2` and `-3` together.** All three concern
  notify-type handling, `3.10.1-2` is already classified provable, and `3.10.1-1` is
  small ("an implementation receiving an unrecognized error type in a response MUST
  assume that the corresponding request has failed entirely").
- If that is refused, `3.10.1-1` and `-2` become -4 and -5, which puts them out of
  document order and contradicts Appendix A. That is the renumbering the brief says has
  already cost this session twice.

---

## 7. Risks

### Amplification, measured rather than asserted

| Emission | Attacker cost | Ze emits | Ratio | Target |
|---|---|---|---|---|
| INVALID_IKE_SPI, unprotected, out-of-SA | 28-byte IKE header, 56 bytes on the wire with UDP and IPv4 | 28-byte header plus an 8-byte notify = 36 bytes, 40 on port 4500 with the marker | **~1.3x on payload, ~0.64x on the wire. It is not an amplifier** | `pkt.RemoteAddr`, so spoofable |
| Protected error notify | must hold the SA's keys | roughly 80 to 120 bytes | ~1x | the configured peer only |
| Cached-response replay, the `2.21.4-5` hole | 28-byte forged header at the last Message ID | the whole cached response; an IKE_AUTH response is typically 300 to 800 bytes | **~10x to 30x** | `sa.remoteUDPAddr()`, the configured peer, so NOT spoofable today |

**The headline is not what the brief assumed, and it is better news.** The
notifications WP-3 ADDS are not amplifiers: the INVALID_IKE_SPI response is smaller
than the request that triggers it. The amplification risk in this package is the one
that already exists, and it is latent only because `sendRaw` still uses the configured
address. WP-8 removes that accident.

### Other risks

| ID | Risk | Early signal | Mitigation |
|---|---|---|---|
| R-WP3-1 | **Over-rejection breaks the strongSwan lab.** Making `ParsePayloadChain` fail on truncation, and adding INVALID_SYNTAX, both turn silent tolerance into a visible refusal. 11 interop scenarios are green today | any of scenarios 01-11 reds | the `2.21.2-1` negative (bad payload CONTENTS must not draw INVALID_SYNTAX) is the guard. `make ze-ipsec-interop-test` runs at the package boundary, not only at the end |
| R-WP3-2 | The `2.5-9` tagged tests break when `ErrUnsupportedCrit` becomes typed | a red in `wire/rfc7296_test.go` or `wire/rfc7296_innerchain_test.go` | **already retired.** All six comparisons in the tree use `errors.Is`, never `==`. Implement `Is()` and there is no behaviour change and no hook trip |
| R-WP3-3 | The `2.1-3` and `2.1-4` tagged tests break when L2 lands | a red in `rfc7296_retransmit_test.go` | the SK-presence guard leaves them green. A full-decrypt guard does not, and is forbidden for that reason |
| R-WP3-4 | Id collision in §2.5 or §3.10.1 | `check_id_allocation` reds the landing commit | Section 6. Land each section's rows together |
| R-WP3-5 | **WP-3 lands after WP-8** and the cached-response hole becomes a live spoofable amplifier | the phase list is reordered | the spec's order already puts WP-3 first. State the dependency in the package's own notes so a reorder is a visible decision |
| R-WP3-6 | INVALID_SYNTAX is wired to the OUTER parse error | a 28-byte forgery draws a guaranteed response | the `3.10.1-3` negative is the mechanical check. It must be written first |
| R-WP3-7 | `2.21.4-6` and `-7` are proven by one test | one tag covers both, and a scope change silently un-proves one | Section 1 records that they govern different messages. Each needs its own fixture, and the tags must name the section scope |

---

## 8. Open questions, for the main thread rather than for me

1. **The package partition moved.** The spec's phase list item 8 calls this WP-3 with
   13 rows, but the later phase 4-15 triage
   (`plan/spec-rfcgate-1b-rfc7296-pilot.md:1540-1560`) re-partitions the work into 14
   packages, WP-4 through WP-17, where WP-4 is "Notify vocabulary and the error-response
   path" (12 rows) and WP-5 is "Unprotected messages, and messages that match no SA" (9
   rows). Those two together are a superset of these 13. The row-by-row mapping lived in
   `tmp/phase4-15-triage.md`, which is gitignored and is gone. **This design covers
   exactly the 13 rows the brief names.** Whether they ship as one package or split
   across the newer WP-4 and WP-5 is a decision I cannot make from the surviving record.
   The split falls naturally at the protected / unprotected boundary of Sections 2 and
   3, so either shape works.

2. **`responder.go:424-429` piggybacking.** Making the IKE_AUTH failure path send
   IDr, CERT, AUTH and the error notify while KEEPING the IKE SA alive is a real
   behaviour change on the authenticated path, and it is governed by Section 2.21.2
   rather than by any of the 13 rows. It is the correct fix and it is in scope under
   `ai/rules/no-parking.md`, but it deserves to be a named decision rather than a
   side effect.

3. **`RFC7296-1.5-1`** ("the receiving node MUST NOT respond to it because doing so
   could cause a message loop", `rfc/full/rfc7296.txt:1054-1056`) is listed under WP-2
   in the spec's phase list item 5, yet it is the same loop-prevention guard as
   `2.21.4-5` and it names the same failure. It should land with L1 and L2, not
   separately.

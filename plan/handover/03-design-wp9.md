<!-- ste: ignore-file preserved verbatim from a design pass; it quotes RFC 7296 at length. -->

# WP-9 design -- Configuration payload, remote access

Rows (17): `RFC7296-2.19-1..-6`, `2.20-1`, `3.15.1-1..-7`, `4-2`, `4-3`, `1.7-1`.
Source spec: the rfcgate-1b RFC 7296 pilot spec, phase list item 15.

**Naming collision, read this first.** The 2026-07-30 re-triage renumbered the work
packages. "WP-9" now names "Crypto suite policy and management facility" (6 rows), and
these 17 rows sit in the new **WP-11, "Configuration payload and remote access"**
(the rfcgate-1b RFC 7296 pilot spec). The brief and the phase list use the
OLD numbering. The rows are what matter. Say which numbering a commit message uses.

**Read-only design.** No tracked file was modified. Every `file:line` below was read in
the working tree on 2026-07-31, and every citation names the FUNCTION as well as the line,
because a second agent is editing `internal/component/ike/engine/` and line numbers move.
Re-locate by function name before quoting a line in a tag.

---

## 0. Verdict

| Row | Appendix A class | This design's verdict | Needs production code? |
|-----|------------------|-----------------------|------------------------|
| `RFC7296-2.19-1` | NOT IMPL | **conformant by non-participation.** Ze is not an IRAC. §4 makes the IRAC role optional | no (owner item OI-1) |
| `RFC7296-2.19-2` | NOT IMPL | **absent.** Becomes live the moment Ze emits CFG_REPLY | **yes** |
| `RFC7296-2.19-3` | NOT IMPL | **absent, and live today in a way the row does not advertise.** Ze already has multiple IKE_AUTH exchanges (EAP) | **yes** |
| `RFC7296-2.19-4` | NOT IMPL | **conformant by non-participation.** Sender obligation on the IRAC. Tied to `2.19-1` | no (owner item OI-1) |
| `RFC7296-2.19-5` | NOT IMPL | **vacuously conformant, becomes the package's primary AUTHORIZATION guard** | **yes** |
| `RFC7296-2.19-6` | NOT IMPL | **absent.** No config leaf expresses it; `NotifyFailedCPRequired` has zero referents | **yes** |
| `RFC7296-2.20-1` | NOT IMPL | **conformant.** Ze sends no CP payload, which is one of the two permitted answers | no |
| `RFC7296-3.15.1-1` | NOT IMPL | **absent.** Netmask cardinality and pairing, both directions | **yes** |
| `RFC7296-3.15.1-2` | NOT IMPL | **conformant by non-participation.** Sender obligation on the requester | no (receive-side tolerance only) |
| `RFC7296-3.15.1-3` | NOT IMPL | **absent.** `SUPPORTED_ATTRIBUTES` (type 14) is not even a constant | **yes** (owner item OI-3) |
| `RFC7296-3.15.1-4` | NOT IMPL | **absent, and actively broken by a codec defect.** The R bit is folded into the attribute type | **yes** |
| `RFC7296-3.15.1-5` | NOT IMPL | **conformant by the RFC's own MAY** (`:6477`), if Ze records the decision to ignore CFG_SET | no (owner item OI-2) |
| `RFC7296-3.15.1-6` | NOT IMPL | **conformant**, same MAY | no (owner item OI-2) |
| `RFC7296-3.15.1-7` | NOT IMPL | **conformant**, same MAY, and directly satisfied by it | no (owner item OI-2) |
| `RFC7296-4-2` | NOT IMPL | **vacuously conformant** (Ze does not support responding). WP-9 makes it live | **yes** |
| `RFC7296-4-3` | NOT IMPL | **vacuously conformant**, same | **yes** |
| `RFC7296-1.7-1` | NOT IMPL | **absent.** And it is NOT a duplicate of `3.15.1-4` -- see section 6 | **yes** |

**Nine rows need production code. Eight are conformant today**, five of them by an
RFC-sanctioned choice rather than by accident. **This is a feature build, not a
test-writing exercise** -- see section 11 for the phase breakdown and the honest estimate.

---

## 1. The finding that reframes this package

The spec says `wire.PayloadCP` is "a dead codec" with "no consumer and no producer"
(the rfcgate-1b RFC 7296 pilot spec). **That is true, and it is only half the
story. The other half is worse and better at the same time.**

An exhaustive grep (28 hits over `internal/`, `cmd/`, `pkg/`, `test/`, the retired `scripts/` (current producer: `internal/le/`))
confirms the codec claim: the only non-test construction of `PayloadCP` is the generic
decoder arm `case PayloadTypeCP: p = &PayloadCP{}` in FUNCTION `decodePayload`
(`internal/component/ike/wire/payload.go`). Nothing type-switches on it. Nothing
builds one.

**But the rest of the feature already exists and is also dead.** Reading the producing
functions:

| Layer | State | Producing function, `file:line` |
|-------|-------|--------------------------------|
| YANG | **exists** -- `list pool { name; range; range6; dns; domain; }`, whose own description cites "IKEv2 Configuration Payload, RFC 7296 Section 2.19" | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` |
| Config parse | **exists** | FUNCTION `parseVirtualIPPool`, `internal/component/ike/ipsec/config.go`; FUNCTION `parseRemoteAccess`, `:474` |
| Config struct | **exists** -- `VirtualIPPool{Name, Range, Range6, DNS, Domain}` | `internal/component/ike/ipsec/types.go` |
| Validation | **exists and is wired** -- IPv4 `/8../30`, IPv6 `/48../126` | FUNCTION `validatePoolPrefix`, `internal/component/ike/ipsec/validate.go`, called from FUNCTION `ValidateRemoteAccess`, `:190`, reached from `internal/component/ike/engine/config.go` |
| Address pool | **exists, mutex-safe, with `Allocate`, `Release`, `Available`** | `internal/core/eap/pool.go`, `:126`, `:191` |
| Pool construction | **exists and RUNS on every config apply** | `internal/component/ike/engine/register.go`, inside FUNCTION `runEngine`'s `OnConfigure` closure |
| Pool consumption | **NONE.** The object is explicitly discarded | **`internal/component/ike/engine/register.go`: `_ = ipPool`** |
| Notify constants | **exist, zero referents each** | `NotifyInternalAddressFailure = 36` and `NotifyFailedCPRequired = 37`, `internal/component/ike/wire/payload_notify.go` |

`_ = ipPool` is the single line that proves the feature is inert. An operator can today
configure a virtual IP pool, have it parsed, validated against prefix bounds, constructed
into a live `eap.Pool`, and logged as `"ike: virtual IP pool created"`
(`register.go`) -- and no client will ever receive an address, because the pool is
blanked to satisfy the compiler four lines before the engine shuts down.

**This is a shipped config surface with no behaviour**, which is what
`ai/rules/completion.md` exists to prevent and what
`ai/rules/completion.md` calls "an inert config surface ... wire it, delete it, or reject
the config -- pick one and do it".

**What it changes for WP-9.** The package is not "build virtual IP assignment from
scratch". It is "join two dead halves through a consumer that does not exist, and fix
what is wrong in each half on the way". The YANG is mostly there. The pool is mostly
there. The codec is mostly there. **The missing piece is `engine/cp.go` and the three
call sites that must feed it.**

### Corroborating comment, not relied upon

`internal/component/ike/engine/config.go` says the remote-access gateway surface
"is inert today and is owned by `plan/spec-ipsec-remote-access.md`". Per
`ai/rules/evidence.md` a comment is its author's belief, not a decision record. It
is cited here only because it agrees with what the code does. **Read
`plan/spec-ipsec-remote-access.md` and `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` before
implementing** -- neither was read in this pass, and either may already hold design
decisions this document would otherwise re-litigate. That is risk R-WP9-9.

---

## 2. §2.19 -- Requesting an internal address (6 rows)

### 2.1 `RFC7296-2.19-1` -- the IRAC must request in IKE_AUTH

> "Since the IKE_AUTH exchange creates an IKE SA and a Child SA, the IRAC MUST request
> the IRAS-controlled address (and optionally other information concerning the protected
> network) in the IKE_AUTH exchange."

`rfc/full/rfc7296.txt:3119-3122`. The MUST is on `:3120`.

**The antecedent is optional by the RFC's own words.** §4 Conformance Requirements:

> "Implementations are not required to support requesting temporary IP addresses or
> responding to such requests."

`rfc/full/rfc7296.txt:6860-6861`.

**What Ze does today.** FUNCTION `buildAuthRequest`
(`internal/component/ike/engine/auth.go`) builds the initiator's IKE_AUTH: IDi,
optional CERTREQ or CERT+AUTH, INITIAL_CONTACT notify, then SAi2/TSi/TSr. **No CP payload
is constructed anywhere in the tree** (section 1). Ze never takes the IRAC role.

**Verdict: conformant by non-participation.** Ze is not an IRAC, so the obligation's
subject does not exist. This is RFC-sanctioned, not a scope reduction.

**But it is a decision, and it must be proven rather than asserted.** The row goes live
the instant anyone adds a client-side virtual-IP request. The tagged pair therefore
asserts the property the code HAS (no builder emits CP) rather than the absence of a
guard, which is the shape that expired for `RFC7296-3.4-1`
(the rfcgate-1b RFC 7296 pilot spec).

**Owner item OI-1.** Implementing the IRAC role is on the table, and
`ai/rules/rfc-compliance.md` forbids me from choosing the narrower answer. See section 14.

### 2.2 `RFC7296-2.19-2` -- CP before SA

> "In all cases, the CP payload MUST be inserted before the SA payload."

`rfc/full/rfc7296.txt:3143`. Appendix A quotes this exactly. No change needed.

**What Ze does today.** FUNCTION `buildAuthResponse`
(`internal/component/ike/engine/responder.go`) builds the inner chain at `:641-651`:

    inner := make([]wire.PayloadEntry, 0, 6)
    inner = append(inner, wire.PayloadEntry{Payload: buildIDPayload(sa, false)})
    if sa.PeerCfg.Auth.Mode == ipsec.AuthX509 {
        inner = append(inner, buildCertPayloads(sa)...)
    }
    inner = append(inner,
        wire.PayloadEntry{Payload: authPayload},
        wire.PayloadEntry{Payload: saPayload},
        wire.PayloadEntry{Payload: respTSi},
        wire.PayloadEntry{Payload: respTSr},
    )

Order: IDr, [CERT...], AUTH, SAr2, TSi, TSr. No CP. `buildEncryptedMessageEx`
(`internal/component/ike/engine/auth.go`) chains `NextPayload` in slice order, so the
slice order IS the wire order.

**Verdict: absent.** Design: the CP entry is appended **between `responder.go`
(AUTH) and `:648` (SAr2)**, which requires splitting the single variadic append. Raise
the capacity hint at `:641` from 6 to 7 so the insert does not realloc.

**Note the asymmetry, and do not "fix" it.** Ze's own parser does not enforce payload
order, deliberately: the comment at `responder.go` says "RFC 7296 Section 2.5
forbids rejecting a message over payload order, so this walk only collects and every
check runs after it." `RFC7296-2.5-13` ("Implementations MUST NOT reject as invalid a
message with those payloads in any other order") is already gated. **`2.19-2` is a SEND
obligation only.** An implementer who adds a receive-side order check to "enforce"
`2.19-2` violates `2.5-13`. Section 12 makes this a guarded invariant.

### 2.3 `RFC7296-2.19-3` -- multiple IKE_AUTH exchanges

> "In variations of the protocol where there are multiple IKE_AUTH exchanges, the CP
> payloads MUST be inserted in the messages containing the SA payloads."

`rfc/full/rfc7296.txt:3144-3146`.

**This row is not hypothetical for Ze. EAP is exactly that variation, and Ze implements
it.** FUNCTION `startResponderEAP` (`internal/component/ike/engine/responder_eap.go`)
and FUNCTION `handleResponderEAP` run the multi-round EAP flow, and the SA
payload appears only in the FINAL IKE_AUTH response, built by `buildAuthResponse` with
`fromEAP == true`.

**What Ze does today: three drop sites, and the third is the one that matters.**

| # | Site | Producing function | What it does with a CP payload |
|---|------|--------------------|--------------------------------|
| 1 | `responder.go` | `handleAuthRequest` | Type switch with cases for `PayloadID`, `PayloadAUTH`, `PayloadCERT`, `PayloadNotify`, `PayloadSA`, `PayloadTS`. **No `*wire.PayloadCP` case, no `default:` arm.** A CP payload is collected into `inner`, matches nothing, and is garbage-collected |
| 2 | `responder_eap.go` | `startResponderEAP` | Receives `remoteSAi2, tsi, tsr` already extracted by site 1. It does not re-walk the chain, so a CP seen in the first EAP IKE_AUTH must be threaded through this signature |
| 3 | `responder_eap.go` | `handleResponderEAP` | Walks `inner` with **only** `case *wire.PayloadEAP:` and `case *wire.PayloadAUTH:`. **A CP payload in the final, post-EAP-Success IKE_AUTH request is dropped here.** This is the site a strongSwan road warrior actually uses |

**Verdict: absent.** Design: all three sites gain CP handling, and site 3 is the one whose
omission would make the feature silently not work against real clients. This is the
second-producer shape the spec has now recorded three times
(the rfcgate-1b RFC 7296 pilot spec), and here it is a THIRD-producer
shape.

**The placement rule that falls out.** In an EAP session the CFG_REPLY goes in the FINAL
IKE_AUTH response (the one carrying SAr2), not the first. In a non-EAP session there is
only one, so the two rules coincide. The design must derive placement from "the message
carrying the SA payload", never from "the first response", or EAP breaks.

### 2.4 `RFC7296-2.19-4` -- CFG_REQUEST content

> "CP(CFG_REQUEST) MUST contain at least an INTERNAL_ADDRESS attribute (either IPv4 or
> IPv6) but MAY contain any number of additional attributes the initiator wants returned
> in the response."

`rfc/full/rfc7296.txt:3148-3150`. Appendix A's parenthetical restructuring ("either IPv4
or IPv6") preserves the meaning; keep it.

**Verdict: conformant by non-participation**, identical disposition to `2.19-1`. Ze sends
no CFG_REQUEST. If OI-1 is answered "implement the IRAC role", this row becomes a send-side
obligation on the new builder and its pair moves from an absence proof to a content
assertion.

**Do not confuse this with `4-2`.** `2.19-4` binds the SENDER of a CFG_REQUEST. `4-2`
binds the RECEIVER to recognize the same attribute. Ze owes `4-2` and not `2.19-4`. The
two tags must say which side they are on, or a reviewer cannot tell them apart -- the
`1.4.1-4` trap (the rfcgate-1b RFC 7296 pilot spec).

### 2.5 `RFC7296-2.19-5` -- no CFG_REPLY without a CFG_REQUEST (AUTHORIZATION)

> "The responder MUST NOT send a CFG_REPLY without having first received a
> CP(CFG_REQUEST) from the initiator, because we do not want the IRAS to perform an
> unnecessary configuration lookup if the IRAC cannot process the REPLY."

`rfc/full/rfc7296.txt:3176-3179`.

**Verdict: vacuously conformant today** (Ze sends no CFG_REPLY at all), and it **becomes
the package's primary authorization guard** the moment WP-9 lands. Full design in
section 8.1.

### 2.6 `RFC7296-2.19-6` -- FAILED_CP_REQUIRED (AUTHORIZATION)

> "In the case where the IRAS's configuration requires that CP be used for a given
> identity IDi, but IRAC has failed to send a CP(CFG_REQUEST), IRAS MUST fail the
> request, and terminate the Child SA creation with a FAILED_CP_REQUIRED error."

`rfc/full/rfc7296.txt:3181-3184`. The sentence continues, and the continuation is
load-bearing:

> "The FAILED_CP_REQUIRED is not fatal to the IKE SA; it simply causes the Child SA
> creation to fail. The initiator can fix this by later starting a new Configuration
> payload request. There is no associated data in the FAILED_CP_REQUIRED error."

`rfc/full/rfc7296.txt:3184-3188`.

**What Ze does today.**

- No config leaf anywhere expresses "CP is required for this identity". The YANG
  `remote-access` container (`ze-ipsec-conf.yang`) has `ike-group`, `esp-group`,
  `authentication`, `list pool`, `list eap-user` and nothing else.
- `NotifyFailedCPRequired uint16 = 37` (`internal/component/ike/wire/payload_notify.go`)
  has **zero referents**, test or production.
- FUNCTION `buildAuthResponse` installs the Child SA at `responder.go` via
  `createFirstChildSA`, **before** the payload list is built at `:641`. A CP-required
  failure that runs after `:623` would leak an installed Child SA and derived keys.

**Verdict: absent.** Needs a new YANG leaf, a new guard, the first use of notify 37, and
a short-circuit ordered before `responder.go`. Full design in section 8.2.

---

## 3. §2.20 -- Requesting the peer's version (1 row)

### `RFC7296-2.20-1`

> "An IKE implementation MAY decline to give out version information prior to
> authentication or even after authentication in case some implementation is known to
> have some security weakness. In that case, it MUST either return an empty string or no
> CP payload if CP is not supported."

`rfc/full/rfc7296.txt:3206-3210`. The MUST is on `:3209`. Appendix A quotes only the
second sentence, which loses the antecedent ("In that case" refers to declining). **The
row text should restore enough of the first sentence to make "that case" resolvable.**
Without it a reader cannot tell what triggers the obligation.

**What Ze does today.** Ze constructs no `PayloadCP` (section 1), so it returns no CP
payload. That is one of the two permitted answers, verbatim.

**Verdict: conformant.** And it stays conformant under either branch of OI-3: if WP-9
declines APPLICATION_VERSION entirely, "no CP payload" continues to hold; if WP-9 answers
it, the guard is "return an empty-valued attribute when declining, never a partial or
placeholder string".

**The trap to avoid.** A well-meaning implementer who returns
`APPLICATION_VERSION("ze")` when the operator has NOT configured a version string has
violated this row: an unconfigured version is a decline, and a decline must be empty or
absent, never a default. The `application-version` YANG leaf therefore has **no default**
(section 7.3), and its zero value means "decline", which is the only reading in which the
zero value is a correct answer rather than a fail-open one.

---

## 4. §3.15.1 -- Configuration attributes (7 rows)

The seven rows split cleanly into two groups: `-1` through `-4` govern CFG_REQUEST /
CFG_REPLY, which WP-9 implements; `-5` through `-7` govern CFG_SET / CFG_ACK, which the
RFC explicitly permits ignoring.

### 4.1 `RFC7296-3.15.1-1` -- one netmask, only with an address

> "INTERNAL_IP4_NETMASK - The internal network's netmask. Only one netmask is allowed in
> the request and response messages (e.g., 255.255.255.0), and it MUST be used only with
> an INTERNAL_IP4_ADDRESS attribute."

`rfc/full/rfc7296.txt:6378-6381`. The MUST is on `:6380`.

**What Ze does today.** FUNCTION `ReadFrom` (`payload_cp.go`) enforces **no
cardinality of any kind**: every attribute is appended at `:81` with no comparison
against previous types. Duplicates of any type all survive. There is no consumer to apply
the rule.

**Verdict: absent.** Two-sided design:
- **Send.** The CFG_REPLY builder emits at most one `INTERNAL_IP4_NETMASK`, and only when
  it also emits an `INTERNAL_IP4_ADDRESS`. The netmask is derived from the pool's
  configured `range` prefix, never from a separate leaf (`ai/rules/evidence.md`).
- **Receive.** A request carrying two netmasks, or a netmask with no address, is not a
  reason to abort -- `3.15.1-4` and `2.5-13` both push toward tolerance. Ze ignores the
  surplus and proceeds. The receive-side rule is "ignore", not "reject".

### 4.2 `RFC7296-3.15.1-2` -- non-empty netmask in a request

> "An empty INTERNAL_IP4_NETMASK attribute can be included in a CFG_REQUEST to request
> this information (although the gateway can send the information even when not
> requested). Non-empty values for this attribute in a CFG_REQUEST do not make sense and
> thus MUST NOT be included."

`rfc/full/rfc7296.txt:6394-6398`. The MUST NOT is on `:6398`.

**Verdict: conformant by non-participation.** Ze sends no CFG_REQUEST. The receive-side
consequence is tolerance: a peer that violates this row by sending a non-empty netmask
must not have its session dropped. Ze ignores the value and treats the attribute as the
request it was meant to be.

### 4.3 `RFC7296-3.15.1-3` -- SUPPORTED_ATTRIBUTES must be zero-length

> "SUPPORTED_ATTRIBUTES - When used within a Request, this attribute MUST be zero-length
> and specifies a query to the responder to reply back with all of the attributes that it
> supports. The response contains an attribute that contains a set of attribute
> identifiers each in 2 octets. The length divided by 2 (octets) would state the number
> of supported attributes contained in the response."

`rfc/full/rfc7296.txt:6425-6431`. The MUST is on `:6426`.

**What Ze does today.** `SUPPORTED_ATTRIBUTES` is attribute type 14. **The constant does
not exist.** `payload_cp.go` defines nine constants -- 1, 2, 3, 4, 6, 7, 8, 10, 12
-- and omits 13 (`INTERNAL_IP4_SUBNET`), 14 (`SUPPORTED_ATTRIBUTES`) and 15
(`INTERNAL_IP6_SUBNET`).

**Verdict: absent.** This row is a SENDER obligation on the requester, so Ze's send side
is conformant by non-participation like `3.15.1-2`. The live question is whether Ze, as
responder, ANSWERS a SUPPORTED_ATTRIBUTES query. That is **owner item OI-3**: answering it
is strictly more compliance and needs no permission; declining it is narrower and does.

If answered, the derivation must come from the constant table itself, not a hand-written
list (`ai/rules/evidence.md`): a `supportedAttributes()` accessor over the set
Ze actually handles, so the reply cannot drift from the handler.

### 4.4 `RFC7296-3.15.1-4` -- unrecognized attributes MUST be ignored

> "If an attribute in the CFG_REQUEST Configuration payload is not zero-length, it is
> taken as a suggestion for that attribute. The CFG_REPLY Configuration payload MAY return
> that value, or a new one. It MAY also add new attributes and not include some requested
> ones. Unrecognized or unsupported attributes MUST be ignored in both requests and
> responses."

`rfc/full/rfc7296.txt:6458-6463`. The MUST is on `:6462-6463`.

**What Ze does today, and the defect this exposes.** FUNCTION `ReadFrom` does no
filtering: `atype` is read raw at `:73` and stored verbatim at `:81`; the `CPAttr*`
constants are never consulted by the parser. Type 0, 5, 65535 and everything between
round-trip into `Attrs`. That is a *reasonable codec design* -- the codec preserves, the
consumer decides -- except that **there is no consumer**, so nothing ignores anything and
nothing acts on anything.

**But there is a live defect underneath it.** §3.15.1 defines the attribute header as a
1-bit Reserved field plus a 15-bit type:

> "Reserved (1 bit) - This bit MUST be set to zero and MUST be ignored on receipt."

`rfc/full/rfc7296.txt:6317-6318`, and:

> "Attribute Type (15 bits) - A unique identifier for each of the Configuration
> Attribute Types."

`rfc/full/rfc7296.txt:6320-6321`.

FUNCTION `ReadFrom` reads **all sixteen bits** into the type:

    atype := binary.BigEndian.Uint16(data[off:])     // payload_cp.go

So a peer that sets the Reserved bit on `INTERNAL_IP4_ADDRESS` yields `atype == 0x8001`
instead of `1`. Ze would classify a perfectly ordinary address request as an unrecognized
attribute and, once a consumer exists, ignore it. **That is a violation of
`RFC7296-2.5-7`** ("The content of all fields marked RESERVED MUST be ignored by an
implementation running version 2.0", `rfc/full/rfc7296.txt` §2.5, already gated) and it
silently defeats `3.15.1-4` and `4-2` at the same time.

The symmetric write-side hazard is at `:49`:
`binary.BigEndian.PutUint16(buf[off+n:], p.Attrs[i].Type)` writes all sixteen bits, so a
`Type` with the high bit set would emit a non-zero Reserved bit, violating
`RFC7296-2.5-6`. No current constant exceeds 12, so the write side is safe in practice
today and unguarded in principle.

**Verdict: absent, and blocked on a codec fix.** Design in section 7.1, defect D1/D2.

### 4.5 `RFC7296-3.15.1-5`, `-6`, `-7` -- the CFG_SET / CFG_ACK trio

> "The CFG_SET and CFG_ACK pair allows an IKE endpoint to push configuration data to its
> peer. In this case, the CFG_SET Configuration payload contains attributes the initiator
> wants its peer to alter. The responder MUST return a Configuration payload if it
> accepted any of the configuration data, and the Configuration payload MUST contain the
> attributes that the responder accepted with zero-length data. Those attributes that it
> did not accept MUST NOT be in the CFG_ACK Configuration payload. If no attributes were
> accepted, the responder MUST return either an empty CFG_ACK payload or a response
> message without a CFG_ACK payload. There are currently no defined uses for the
> CFG_SET/CFG_ACK exchange, though they may be used in connection with extensions based
> on Vendor IDs. **An implementation of this specification MAY ignore CFG_SET payloads.**"

`rfc/full/rfc7296.txt:6465-6477`. The three MUSTs are at `:6468-6470` (`-5`), `:6471-6472`
(`-6`) and `:6472-6474` (`-7`). **The MAY at `:6477` is the disposition for all three.**

**What Ze does today.** Ze accepts no configuration data from any peer, because there is
no CP consumer at all. So it accepts nothing, sends no CFG_ACK, and returns "a response
message without a CFG_ACK payload".

**Verdict: all three conformant**, and `3.15.1-7` is satisfied *by name* -- "a response
message without a CFG_ACK payload" is literally what Ze does.

**This is a decision, not an accident, and it must be recorded as one.** The design
choice is: **Ze ignores CFG_SET payloads, exercising the MAY at `:6477`.** With that
recorded, the three obligations become:

- `-5`: antecedent false. Ze accepts no configuration data, so "if it accepted any" never
  fires.
- `-6`: Ze sends no CFG_ACK, so no unaccepted attribute can be in one.
- `-7`: satisfied affirmatively by the response-without-CFG_ACK behaviour.

Each is provable against a property the code HAS -- "a CFG_SET produces no CFG_ACK and no
state change" -- rather than against an absent guard. That is the distinction that made
`RFC7296-3.1-11`'s pair durable and `RFC7296-3.4-1`'s expire
(`plan/handover/02-design-wp5.md`, section 2.4).

**Owner item OI-2.** Exercising an explicit RFC MAY is not a deviation, but it is a
choice that lowers nothing only if it is deliberate. Section 14.

---

## 5. §4 -- Conformance requirements (2 rows)

Both rows are conditional on a support decision Ze has not yet made, and **WP-9 is the
decision.**

### 5.1 `RFC7296-4-2` -- parse the CFG_REQUEST and recognize the address attribute

> "If an implementation supports responding to such requests, it MUST parse the CP
> payload of type CFG_REQUEST in the first message in the IKE_AUTH exchange and recognize
> a field of type INTERNAL_IP4_ADDRESS or INTERNAL_IP6_ADDRESS."

`rfc/full/rfc7296.txt:6866-6869`.

### 5.2 `RFC7296-4-3` -- return a CFG_REPLY with an address of the requested type

> "If it supports leasing an address of the appropriate type, it MUST return a CP payload
> of type CFG_REPLY containing an address of the requested type. The responder may
> include any other related attributes."

`rfc/full/rfc7296.txt:6869-6872`.

**What Ze does today: vacuously conformant.** The antecedent "supports responding to such
requests" is false -- section 1. Both rows are true because their conditions are false.

**WP-9 makes both live.** After WP-9, Ze supports responding, so it MUST parse and MUST
reply. Design: sections 7.2 and 8.

**One reconciliation the implementer must not get wrong.** `4-2` says "in the FIRST
message in the IKE_AUTH exchange"; `2.19-3` says CP goes "in the messages containing the
SA payloads". In an EAP flow these differ on the RESPONSE side but agree on the REQUEST
side: the initiator sends SAi2 in its first IKE_AUTH, so the CFG_REQUEST arrives there;
the responder's SAr2 appears only in the final IKE_AUTH response, so the CFG_REPLY goes
there. **Parse early, reply late.** A design that replies in the first response breaks
EAP; a design that only parses the final request breaks strict `4-2` reading. Ze must
accept a CFG_REQUEST at drop sites 1 and 3 both (section 2.3), and hold it on the SA
until the SA-bearing response is built.

**`4-3`'s "of the requested type" is a real constraint, not a formality.** If the client
requests only `INTERNAL_IP6_ADDRESS` and the pool has only an IPv4 range, Ze must not
answer with an IPv4 address. §3.15.4 gives the correct behaviour:

> "The initiator may request a particular type of address (IPv4 or IPv6) that the
> responder does not support, even though the responder supports Configuration payloads.
> In this case, the responder simply ignores the type of address it does not support and
> processes the rest of the request as usual."

`rfc/full/rfc7296.txt:6674-6678`.

---

## 6. §1.7 -- attribute type 5, and the duplication question the spec asked WP-9 to settle

### The obligation, verbatim

> "This document removes discussion of the INTERNAL_ADDRESS_EXPIRY configuration
> attribute because its implementation was very problematic. Implementations that conform
> to this document MUST ignore proposals that have configuration attribute type 5, the
> old value for INTERNAL_ADDRESS_EXPIRY."

`rfc/full/rfc7296.txt:1147-1151`. The MUST is on `:1149-1151`.

### The erratum

Erratum 5056 (Held for Document Update, Technical) reports that "proposals" is wrong: a
configuration attribute belongs to a Configuration payload, not to an SA proposal. The
verifier's words, as recorded by phase 2a: `only the attribute type should be ignored, not
the entire proposal` (the rfcgate-1b RFC 7296 pilot spec).

The spec's constraint is explicit: **the row keeps the verbatim text, and WP-9 implements
the CORRECTED semantics** (`:1284`). Implementing the literal text would discard an entire
SA proposal because a Configuration payload elsewhere in the message carried attribute 5 --
a self-evidently wrong behaviour that would break interoperability.

### The duplication question, settled

The rfcgate-1b RFC 7296 pilot spec asks WP-9 to settle whether `1.7-1`
duplicates `3.15.1-4` ("Unrecognized or unsupported attributes MUST be ignored"), and says
that if they are one obligation the sign-off excludes one site as `duplicate-of` the other.

**They are NOT duplicates. Keep both sites.** The reasoning is mechanical, not stylistic:

| | `RFC7296-3.15.1-4` | `RFC7296-1.7-1` |
|---|---|---|
| Scope | attributes Ze does **not** recognize | attribute type **5**, specifically |
| Mechanism | default-deny over the unknown | a **named denylist** entry |
| Would an implementation that recognizes type 5 satisfy it? | yes, vacuously -- a recognized attribute is outside `3.15.1-4`'s scope | **no.** Type 5 must be ignored *even by an implementation that knows what it is* |
| Failure mode if omitted | an unknown attribute is acted on | a **known, deprecated** attribute is acted on |

Attribute 5 was a defined attribute in RFC 4306. An implementation carrying RFC 4306
heritage recognizes it, so `3.15.1-4` gives it no instruction at all. `1.7-1` is precisely
the rule that catches that case. They are one line apart in the code and two different
obligations.

**What Ze does today.** No consumer, so nothing is ignored and nothing is acted on.
`CPAttrInternalAddressExpiry` (5) is not even declared (`payload_cp.go` jumps 4 → 6).

**Verdict: absent.** Design: declare the constant and have the attribute dispatcher drop
type 5 before the unknown-attribute default arm, so the two rules are visibly distinct
and separately mutable.

---

## 7. Production design

### 7.1 Codec: `internal/component/ike/wire/payload_cp.go`

Eight findings from reading `WriteTo`, `Len` and `ReadFrom`
(`:65-85`). D1, D2, D6 and D7 are required by the rows; the rest are correctness work in
the same file.

| Id | Defect | Site | Fix | Row |
|----|--------|------|-----|-----|
| **D1** | Attribute type read as 16 bits; the Reserved bit is folded into the type | `ReadFrom`, `:73` | mask: `binary.BigEndian.Uint16(data[off:]) &^ 0x8000` | `3.15.1-4`, `4-2`, and `2.5-7` |
| **D2** | Attribute type written as 16 bits; a high-bit type emits a non-zero Reserved bit | `WriteTo`, `:49` | mask on write | `2.5-6` |
| **D3** | `uint16(len(Value))` truncates for a value over 65535 while `:51` copies it all, producing a corrupt encoding | `WriteTo`, `:50-51` | reject or cap at build time; today unreachable because there is no producer, live the moment WP-9 adds one | correctness |
| **D4** | A 1-3 byte trailing remnant is silently discarded rather than reported | `ReadFrom`, `:72` | return `ErrTruncated` | correctness |
| **D5** | `CFGType` is not validated against 1..4 | `ReadFrom`, `:69` | leave the codec permissive; the **consumer** validates (section 8.1). Do not reject in the codec, or `2.5-13`-style tolerance is lost | see 8.1 |
| **D6** | Unbounded attribute count: a 65535-byte body yields roughly 16k `ConfigAttr`, each a heap allocation at `:79` | `ReadFrom`, `:72-83` | cap at 64 attributes; return `ErrTruncated` beyond | resource bound, section 8.3 |
| **D7** | Missing constants: 5 (`INTERNAL_ADDRESS_EXPIRY`, needed as a denylist entry), 13 (`INTERNAL_IP4_SUBNET`), 14 (`SUPPORTED_ATTRIBUTES`), 15 (`INTERNAL_IP6_SUBNET`) | `:16-26` | declare all four | `1.7-1`, `3.15.1-3` |
| **D8** | `WriteTo` does no bounds check on `buf` | `:42-55` | consistent with the package's buffer-first convention (callers size from `Len()`); **note, do not change** without checking sibling payloads | none |

**D5 deserves its own sentence.** It is tempting to reject an out-of-range `CFGType` in
the codec. Do not. The codec's job is to present what arrived; the authorization decision
in section 8.1 is where an unmapped `CFGType` must deny. Rejecting in the codec would turn
a peer's odd-but-harmless payload into a parse failure for the whole message, which
`2.5-13` and `3.15.1-4` both push against.

### 7.2 The consumer: `internal/component/ike/engine/cp.go` (new)

The file the spec's phase list already names (the rfcgate-1b RFC 7296 pilot spec).

| Function | Responsibility | Rows |
|----------|----------------|------|
| `cpRequestFrom(inner []wire.PayloadEntry) (*wire.PayloadCP, bool)` | Find the single CP payload in a decrypted chain. Returns `false` for zero, for two or more, and for any `CFGType` that is not `CFGTypeRequest` | `2.19-5`, `4-2` |
| `wantsAddress(cp *wire.PayloadCP) (v4, v6 bool)` | Recognize `INTERNAL_IP4_ADDRESS` / `INTERNAL_IP6_ADDRESS`, honouring zero-length as "assign me one" | `4-2`, `4-3` |
| `attributeDisposition(t uint16) disposition` | Three-way: `handled`, `deprecated` (type 5 only), `ignored`. Type 5 is checked **before** the unknown default, so `1.7-1` and `3.15.1-4` stay separately mutable | `1.7-1`, `3.15.1-4` |
| `buildCFGReply(lease *eap.AllocateResult, req *wire.PayloadCP, cfg) *wire.PayloadCP` | Assemble the reply: address of the requested type only, at most one netmask and only beside an IPv4 address, DNS, optional `APPLICATION_VERSION` | `4-3`, `3.15.1-1`, `2.20-1` |
| `cpPolicyFor(sa *SA) (required, resolved bool)` | The `2.19-6` policy lookup, with an explicit `resolved` so a miss cannot read as "not required" | `2.19-6` |

**Where the lease is taken, and why the order is forced.** `createFirstChildSA` runs at
`responder.go`, and `buildChildSAResponsePayloads` at `:613` produces the responder's
TSi/TSr. `4-3`'s address must narrow `sa.NegotiatedTSi` to the leased address, so
**allocation must happen before `responder.go`**. Otherwise the responder echoes the
selectors the client proposed (set at `responder.go`) and the leased address never
reaches the traffic selectors, producing a session that negotiates an address and then
routes as if it had not. The `2.19-6` refusal must short-circuit before `:623` for the
same reason: no Child SA, no keys.

### 7.3 Config surface

Everything below attaches under the existing
`vpn/ipsec/remote-access` container (`ze-ipsec-conf.yang`).

**New container**, following `ai/rules/config.md` (kebab-case, no abbreviations,
noun phrases, positive booleans) and `ai/rules/config.md` (`units` on every
dimensioned leaf, protocol-sane defaults, no boolean-as-enum):

    container configuration-payload {
        description "IKEv2 Configuration payload address assignment
            (RFC 7296 Section 3.15).";

        leaf enabled {
            type boolean;
            default false;
            description "Answer a client CP(CFG_REQUEST) with a leased
                address from the pool.";
        }

        leaf required {
            type boolean;
            default false;
            description "Terminate Child SA creation with FAILED_CP_REQUIRED
                when a client sends no CP(CFG_REQUEST). The IKE SA survives.";
        }

        leaf application-version {
            type string;
            description "Version string returned in APPLICATION_VERSION.
                When unset, Ze declines and returns no value.";
        }

        leaf lease-lifetime {
            type uint32;
            units seconds;
            default 3600;
            description "How long a leased address is held after its IKE SA ends.";
        }

        leaf maximum-leases-per-identity {
            type uint16;
            default 1;
            description "Concurrent leases one authenticated identity may hold.";
        }
    }

Naming check: `configuration-payload` not `cp`; `maximum-leases-per-identity` not
`max-leases`; `enabled` and `required` are positive assertions; `lease-lifetime` is
dimensioned and carries `units seconds` with a default, per the Units rule; the
lease-count leaf is a pure count and correctly carries no `units`.

**`application-version` deliberately has no default.** Section 3 explains why: an
unconfigured version is a decline, and `2.20-1` requires a decline to be empty or absent.
A default would make the zero value a wrong answer.

**Two existing-surface fixes the rows force:**

| Fix | Site | Why |
|-----|------|-----|
| `leaf dns` → `leaf-list dns` | `ze-ipsec-conf.yang` | `INTERNAL_IP4_DNS` is multi-valued per `rfc/full/rfc7296.txt:6346`, and `VirtualIPPool.DNS` is already `[]string` (`types.go`). FUNCTION `parseVirtualIPPool` appends a single `t.Get("dns")` at `config.go`, so **only one DNS server can be configured today** while the struct and the RFC both allow many |
| `list pool` -- read all, or reject more than one | FUNCTION `parseRemoteAccess`, `config.go` | It takes `pools[0]` and **silently discards every other pool**. That is an `ai/rules/protocol.md` violation: the operator's config is not applied and nothing says so. Minimum acceptable fix is a validation error naming the count; the better fix is per-peer pool selection |

`range6` is inconsistent with the no-abbreviation convention, but it is existing surface
and renaming it is out of scope for the rows. Flagged, not changed.

### 7.4 Pool: `internal/core/eap/pool.go`

| Id | Finding | Site | Design |
|----|---------|------|--------|
| **P1** | The pool is constructed and then discarded | `runEngine`, `register.go` then `:393` `_ = ipPool` | Hang it off the `PeerSession` / SA-table so `buildAuthResponse` can reach it. `activeTablePtr` (`register.go`) is the existing precedent for engine-scoped shared state |
| **P2** | `Allocate()` takes no identity, so there is no reuse and no per-identity quota | `pool.go` | New signature carrying the authenticated identity. Enables `maximum-leases-per-identity` and address reuse across rekeys, which `rfc/full/rfc7296.txt:6374-6375` calls for ("valid as long as this IKE SA (or its rekeyed successors) ... is valid") |
| **P3** | `Release` has **no non-test caller**, and no lease/expiry concept exists anywhere in `internal/component/ike/` | `pool.go` | A lease table keyed by identity plus SA, released on SA teardown and on `lease-lifetime` expiry. **Without this the pool leaks monotonically until exhaustion under nothing worse than normal client churn** |
| **P4** | `p.size4 = (1 << uint(bits-ones)) - 2` underflows for a very short prefix | `NewPool`, `pool.go` | For a `/0`, `bits-ones` is 32; a `uint32` shift by 32 yields 0 and `0 - 2` wraps to 4294967294, which the guard at `:54` (`p.size4 == 0 \|\| bits-ones < 1`) does not catch. **Unreachable today** because `validatePoolPrefix` bounds IPv4 to `/8../30` (`validate.go`) and that validation IS wired (`engine/config.go`). Defence in depth: `NewPool` must bound itself rather than trust its caller (`ai/rules/evidence.md`, "make the miss explicit at the producer"). **This claim is a reading of the source plus Go's defined shift semantics; it was not executed. The boundary test in section 9 is what settles it** |
| **P5** | `allocateV4` scans linearly from offset 1 on every call | `pool.go` | At the `/8` the validator permits, that is up to 16,777,214 probes and a map of the same order. Add a rotating cursor like the IPv6 path's `next6` |
| **P6** | `allocateV6` writes the host ID into `ip6[8:]` only | `pool.go` | That assumes a prefix no longer than `/64`, but `validatePoolPrefix` permits `/48../126` (`validate.go`). **A `/96` pool clobbers prefix bits and hands out addresses outside the configured range.** Either narrow the validator to `/48../64` or make `allocateV6` prefix-aware |

P6 is a live bug reachable from a valid config today. It has no row of its own, and
`ai/rules/completion.md` makes it in scope the moment WP-9 becomes the entry point that
allocates from this pool.

### 7.5 Files that change

| File | Change |
|------|--------|
| `internal/component/ike/engine/cp.go` | **new** -- the consumer, the producer, and both authorization guards |
| `internal/component/ike/engine/responder.go` | `handleAuthRequest`: a `*wire.PayloadCP` case in the walk at `:375-410`. `buildAuthResponse`: CP inserted between `:647` and `:648`; capacity hint at `:641` 6 → 7; `2.19-6` short-circuit before `:623`; TS narrowing before `:613` |
| `internal/component/ike/engine/responder_eap.go` | `startResponderEAP` signature at `:116` carries the held CP request; `handleResponderEAP` walk at `:201-209` gains a CP case |
| `internal/component/ike/engine/register.go` | `_ = ipPool` at `:393` replaced by real wiring |
| `internal/component/ike/wire/payload_cp.go` | D1, D2, D3, D4, D6, D7 |
| `internal/core/eap/pool.go` | P2, P3, P4, P5, P6 |
| `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` | `container configuration-payload`; `leaf dns` → `leaf-list dns` |
| `internal/component/ike/ipsec/config.go`, `types.go`, `validate.go` | parse, hold and validate the new leaves; the multi-pool decision |
| `internal/core/diagnostic/codes.go` | a `doctor-ipsec-virtual-ip-pool` code (`ai/rules/repo-maintenance.md`: the pool is a config-declared resource) |
| `docs/features.md`, `docs/guide/`, `docs/architecture/wire/`, `docs/features/rfc-status.md` | `ai/rules/writing.md` and `ai/rules/repo-maintenance.md` |

---

## 8. Security, first class

### 8.1 `RFC7296-2.19-5` -- how it fails closed

The guard is `cpRequestFrom`. It denies unless **every** condition holds, and its default
return is `(nil, false)`:

| # | Condition | The miss it closes |
|---|-----------|--------------------|
| 1 | Exactly one `*wire.PayloadCP` in the decrypted inner chain of **this** request | Two CP payloads is ambiguous input. "Take the first" is a silent choice made on attacker-supplied data |
| 2 | `cp.CFGType == wire.CFGTypeRequest` | **The unmapped-input case.** `CFGTypeReply`, `CFGTypeSet`, `CFGTypeACK` and all 251 undefined values deny |
| 3 | At least one `INTERNAL_IP4_ADDRESS` or `INTERNAL_IP6_ADDRESS` attribute present | **The empty-set case.** A CP with zero attributes is present-but-empty |
| 4 | The peer's profile has `configuration-payload/enabled = true` | Policy, resolved explicitly (see 8.2) |

**The zero-value trap, named so the implementer meets it here and not in review.** The
natural Go idiom is:

    var cpReq *wire.PayloadCP
    // ... switch arm sets cpReq ...
    if cpReq != nil { /* build CFG_REPLY */ }

**That fails open on condition 2.** A peer sending `CP(CFG_SET)` -- which the RFC
explicitly permits Ze to ignore, `rfc/full/rfc7296.txt:6477` -- would receive a CFG_REPLY
carrying a leased address it never asked for. `ai/rules/evidence.md` is exact
about this: on "an unmapped input" the guard must deny, and "a present-but-empty value
passes `ok`". A nil test satisfies neither condition 2 nor condition 3.

**The rule for the implementer: the decision is one function returning an explicit
`ok`. There is no nil test at the call site.** If the call site can express
`if cpReq != nil`, the design has already failed.

### 8.2 `RFC7296-2.19-6` -- how it fails closed, and which way "closed" points

This guard is subtler, because the permissive branch and the restrictive branch are not
where instinct puts them. The **permissive** branch is "proceed without CP". The
**restrictive** branch is "fail the Child SA with FAILED_CP_REQUIRED".

`cpPolicyFor(sa *SA) (required, resolved bool)`:

| Input state | `resolved` | Behaviour |
|-------------|-----------|-----------|
| `sa.PeerCfg` is the remote-access profile | true | read `configuration-payload/required` |
| `sa.PeerCfg` is a site-to-site peer | true | `required = false`, **structurally**: the leaf does not exist on that branch and CP is not part of that profile. This is a determined answer, not a miss |
| `sa.PeerCfg` is nil or unresolvable | **false** | **DENY.** Fail the Child SA and log. A session that reached IKE_AUTH response build with no bound profile is a bug state, and a bug state must not take the permissive branch |

**Why `map[string]bool` is banned here.** A policy lookup written as
`required := policy[identity]` returns `false` on a miss, and `false` means "CP not
required", which is the permissive branch. The zero value would read as a valid answer --
exactly what `ai/rules/evidence.md` forbids. The explicit `resolved` return is
what stops it.

**Two behavioural details the RFC pins and an implementer will get wrong.**

1. **The IKE SA survives.** `rfc/full/rfc7296.txt:3184-3186`: "The FAILED_CP_REQUIRED is
   not fatal to the IKE SA; it simply causes the Child SA creation to fail." So the
   response is an IKE_AUTH response carrying `N(FAILED_CP_REQUIRED)` and **no SA and no TS
   payloads**, with the IKE SA established. Tearing down the IKE SA is a violation.
2. **No Child SA may be installed.** The refusal must short-circuit before
   `createFirstChildSA` at `responder.go`, or Ze installs kernel state and derives keys
   for a Child SA it is simultaneously refusing.

`FAILED_CP_REQUIRED` carries no data: "There is no associated data in the
FAILED_CP_REQUIRED error" (`rfc/full/rfc7296.txt:3187-3188`).

### 8.3 The address pool as a resource-exhaustion surface

**What bounds it today, and what does not.**

| Bound | Exists? | Where |
|-------|---------|-------|
| Pool prefix `/8../30` (IPv4), `/48../126` (IPv6) | **yes, and wired** | `validatePoolPrefix`, `validate.go`, via `ValidateRemoteAccess` `:190`, reached from `engine/config.go` |
| IPv6 allocation hard cap of 65536 | **yes** | `allocateV6`, `pool.go` |
| Per-identity lease quota | **no** | `Allocate()` takes no identity (`pool.go`) |
| Lease release on SA teardown | **no** | `Release` has zero non-test callers |
| Lease expiry | **no** | no lease concept exists anywhere in `internal/component/ike/` |
| Attribute count per CP payload | **no** | `ReadFrom`, `pool` is not involved; roughly 16k attributes per 65535-byte body |
| `NewPool` self-bounding | **no** | trusts the caller; P4 |

**The mitigating fact, stated plainly: CP arrives inside SK, after authentication.** The
CFG_REQUEST is in the encrypted inner chain of an IKE_AUTH message, so an attacker must
hold valid credentials to reach the allocator at all. That removes the unauthenticated
DoS and leaves an authenticated one.

**The biggest risk is not an attack. It is the default behaviour.** With P3 unfixed,
every client that ever connects consumes an address permanently. A `/24` pool serving a
50-user site exhausts after 254 connections -- a few days of normal laptop churn -- and
then every subsequent client is refused. No attacker is required. **`Release` wiring plus
`lease-lifetime` expiry is not a hardening extra; it is the difference between a feature
that works for a week and one that works.**

**The authenticated attack, second.** With P2 unfixed, one valid credential can open N
parallel IKE SAs and drain the pool, denying service to every other remote-access user.
`maximum-leases-per-identity` (default 1) is the bound, and address reuse for the same
identity is what `rfc/full/rfc7296.txt:6374-6375` already asks for.

**Required additional bounds:**

- Cap attributes per CP payload at 64 in `ReadFrom` (D6).
- `NewPool` rejects a range it cannot represent, independent of `validatePoolPrefix` (P4).
- A rotating cursor in `allocateV4` so a large pool does not cost a linear scan per
  allocation (P5).
- Fix `allocateV6` for prefixes longer than `/64`, or narrow the validator (P6).
- A `doctor-ipsec-virtual-ip-pool` check reporting configured size and current
  utilisation, so exhaustion is visible before it is total (`ai/rules/repo-maintenance.md`).

### 8.4 One correctness obligation the row set does not contain

§3.15.4 requires `INTERNAL_ADDRESS_FAILURE` when allocation fails:

> "If the responder encounters an error while attempting to assign an IP address to the
> initiator during the processing of a Configuration payload, it responds with an
> INTERNAL_ADDRESS_FAILURE notification. The IKE SA is still created even if the initial
> Child SA cannot be created because of this failure."

`rfc/full/rfc7296.txt:6652-6656`.

**Appendix A has no §3.15.4 row**, so this is not one of WP-9's 17. It is nonetheless
required for the feature to be correct: without it, pool exhaustion produces an
unexplained failure rather than the diagnosable notification the RFC specifies.
`NotifyInternalAddressFailure = 36` already exists with zero referents
(`payload_notify.go`). **Implement it, and raise with the owner whether §3.15.4 should
gain rows** -- an unextracted obligation is still an obligation
(`ai/rules/rfc-compliance.md`, Extraction Completeness). This is owner item OI-4.

---

## 9. Tagged tests and mutations

### 9.1 Where a tag may live

`CARRIERS` (the retired `scripts/dev/rfc_requirements.py` (current producer: `internal/le/rfc/rfc.go`)) admits four carriers, and the choice
is not free:

| Carrier | Cell | Tier | Usable here? |
|---------|------|------|--------------|
| `*_test.go` | `unit/verify` | every push | **yes** |
| `*.ci` in a suite `./le functional` names | `functional/verify` | every push | **yes** -- `ipsec` is in the `all_suites` line (the retired `mk/test-functional.mk` (current producer: `internal/le/functional/suites.go`)) |
| `*.et` | `editor/verify` | every push | not applicable |
| `test/interop/scenarios/*/check.py` | `interop/nightly` | scheduled, advisory | not applicable |

**`test/interop-ipsec/` is REFUSED as a tag carrier.** `ai/rules/testing.md` is explicit:
a tag in `test/interop-ipsec/` "is REFUSED with an error naming the file, because nothing
runs those suites automatically and a tag nothing executes is an absence of evidence
rather than weak evidence".

**Consequence for WP-9.** The strongSwan scenario `remote-access-cp`
(the rfcgate-1b RFC 7296 pilot spec) is **goal-validation evidence, not tagged
evidence**. It proves the feature works against a real peer, and it earns no row a
polarity. Every one of the 34 tags below lives in a `_test.go` or in
`test/ipsec/ipsec-remote-access-cp.ci`. An implementer who tags the interop `check.py`
will fail `./le rfc check` with an error naming the file.

### 9.2 Test homes

| File | Layer | Holds |
|------|-------|-------|
| `internal/component/ike/wire/rfc7296_cp_test.go` (new) | codec | `3.15.1-1..-4`, `1.7-1` codec halves; the R-bit defect |
| `internal/component/ike/engine/cp_test.go` (new) | engine | `2.19-*`, `4-2`, `4-3`, `2.20-1`, `3.15.1-5..-7`; the spec names `TestConfigurationPayloadExchange` here |
| `internal/core/eap/pool_test.go` (exists) | pool | P4 boundary, P6 prefix, quota and release |
| `test/ipsec/ipsec-remote-access-cp.ci` (new) | functional | the operator path end to end |
| `test/interop-ipsec/scenarios/remote-access-cp/` (new) | interop | goal validation only, **no tags** |

### 9.3 The pairs, and the mutation that must redden each

| Row | Positive asserts | Negative asserts | Mutation that must redden |
|-----|------------------|------------------|---------------------------|
| `2.19-1` | Over every message Ze builds as initiator, no payload is a `*wire.PayloadCP` | The builder CAN carry one: a `PayloadEntry{Payload: &wire.PayloadCP{}}` inserted into `buildAuthRequest`'s list does reach the wire, so the absence is a decision, not an encoder limit | add a CP entry in `buildAuthRequest` (`auth.go`) → positive reddens |
| `2.19-2` | In every IKE_AUTH response carrying both, the CP index is strictly less than the SA index | The harness can tell the order apart: a fixture with CP after SA is detected by the same assertion | swap the two appends at `responder.go` → positive reddens |
| `2.19-3` | In an **EAP** session, the CFG_REPLY appears in the response carrying SAr2 (the final IKE_AUTH), not the first | In a non-EAP session the same rule puts it in the only response. Assert both flows in one test so "the message carrying SA" is proven to be the rule, not "the first response" | make the reply site "first response" → the EAP half reddens, the non-EAP half stays green. **That asymmetry is the proof the negative exists for** |
| `2.19-4` | Ze emits no CFG_REQUEST at all (shares `2.19-1`'s sweep) | Tag states the side: this is a SENDER obligation Ze does not incur, distinct from `4-2` which it does | same as `2.19-1` |
| `2.19-5` | A request with **no** CP gets a response with no CP. A request whose CP is `CFGTypeSet`, `CFGTypeReply` or `CFGTypeACK` gets a response with no CP. A CP with zero attributes gets none | A well-formed `CFGTypeRequest` **does** get a CFG_REPLY, so the guard is not refusing everything | in `cpRequestFrom`, drop the `CFGType` check → the CFG_SET case reddens. Drop the attribute-presence check → the empty-CP case reddens. **Run both separately** |
| `2.19-6` | `required=true` plus no CP ⇒ response carries `N(FAILED_CP_REQUIRED)`, **no SA payload, no TS payload**, and the IKE SA is still `StateEstablished`, and **no Child SA was installed** | `required=false` plus no CP ⇒ normal establishment. And `required=true` plus a valid CP ⇒ normal establishment. The guard fires on the conjunction, not on either half | make `cpPolicyFor` return `required` from a bare map lookup → the unresolved-profile case must redden. Move the short-circuit after `responder.go` → the "no Child SA installed" assertion reddens |
| `2.20-1` | With `application-version` unset, no `APPLICATION_VERSION` attribute is emitted, in any exchange | With it set, the configured string IS emitted, so the absence is a decline rather than a missing code path | make the unset case emit `APPLICATION_VERSION("ze")` → positive reddens |
| `3.15.1-1` | A CFG_REPLY carries at most one netmask, and carries one only when it also carries `INTERNAL_IP4_ADDRESS` | An IPv6-only lease produces **no** netmask, so the pairing is conditional rather than always-on | emit the netmask unconditionally in `buildCFGReply` → the IPv6-only case reddens |
| `3.15.1-2` | Ze emits no CFG_REQUEST, so it emits no non-empty netmask in one | Receive-side tolerance: a request carrying a **non-empty** netmask still yields a normal CFG_REPLY and no teardown | make the consumer error on a non-empty request netmask → negative reddens |
| `3.15.1-3` | Per OI-3. If answered: a zero-length `SUPPORTED_ATTRIBUTES` request yields a reply whose value length is a multiple of 2 and whose contents are derived from the handler set. If declined: no `SUPPORTED_ATTRIBUTES` is emitted | If answered: a **non**-zero-length `SUPPORTED_ATTRIBUTES` in a request is ignored rather than answered | hardcode the supported list instead of deriving it, then remove an attribute from the handler → the derived assertion reddens, a hardcoded one would not |
| `3.15.1-4` | A CFG_REQUEST containing types 9, 11, 99 and 65535 alongside `INTERNAL_IP4_ADDRESS` still yields a correct CFG_REPLY; the unknown types appear in neither the reply nor any state | **The R-bit case.** `INTERNAL_IP4_ADDRESS` with the Reserved bit set (`0x8001`) is recognized as type 1 and answered. This is the D1 fix, and it is the half that gates a real defect | revert the D1 mask at `payload_cp.go` → the negative reddens and the positive stays green |
| `3.15.1-5` | A `CP(CFG_SET)` in an INFORMATIONAL exchange produces a response with **no** CP payload and **no** configuration state change | The same exchange **does** produce a response (the request is answered, just without a CFG_ACK), so the absence is a choice rather than a dropped message | make the CFG_SET path emit an empty `CFGTypeACK` → positive reddens |
| `3.15.1-6` | No CFG_ACK is ever built, so no unaccepted attribute can appear in one | The builder could produce one: a hand-built `CFGTypeACK` encodes and decodes, so the absence is a decision | shared with `3.15.1-5` |
| `3.15.1-7` | A `CP(CFG_SET)` yields "a response message without a CFG_ACK payload", the RFC's own second option | The response exists and is well-formed | shared with `3.15.1-5` |
| `4-2` | A CFG_REQUEST in the **first** IKE_AUTH request is parsed and its `INTERNAL_IP4_ADDRESS` / `INTERNAL_IP6_ADDRESS` recognized, in **both** the direct and the EAP flow | A CFG_REQUEST arriving only in the **final** EAP IKE_AUTH is also recognized (drop site 3, `responder_eap.go`) | remove the CP case from `handleAuthRequest` → positive reddens. Remove it from `handleResponderEAP` → **negative only** reddens, which is what proves the third producer is covered |
| `4-3` | A client requesting IPv4 receives an IPv4 address from the configured range; a client requesting IPv6 receives IPv6 | **Type fidelity.** A client requesting **only** IPv6 against an IPv4-only pool receives no address of the wrong family, and the session still completes per `rfc/full/rfc7296.txt:6674-6678` | make `buildCFGReply` answer with whatever the pool has → negative reddens |
| `1.7-1` | Attribute type 5 in a CFG_REQUEST is ignored: it appears in no reply and changes no state | **The scope proof.** The rest of the payload, and the SA proposal in the same message, are processed normally. The literal reading ("ignore proposals") would discard the proposal; the corrected reading discards one attribute | make `attributeDisposition` return `handled` for 5 → positive reddens. Make the type-5 branch abort the message → negative reddens |

**Two anti-vacuity guards are mandatory**, on the pattern that WP-5 made a requirement:

1. **`2.19-5`'s positive can pass over a sample containing no CP at all.** The test MUST
   assert the sample contains at least one request WITH a CP payload and at least one
   WITHOUT, and name both counts in the failure message. Otherwise "no CFG_REPLY without a
   CFG_REQUEST" is proven over a set where no CFG_REQUEST exists.
2. **`4-2`'s positive can pass over the direct flow only.** The test MUST assert it
   observed a CP in both an EAP and a non-EAP session, because drop site 3 is reachable
   only through EAP.

### 9.4 Pool tests

Not row-tagged (they prove no RFC obligation), but required by
`ai/rules/completion.md` for the code WP-9 changes:

| Test | Asserts | Mutation |
|------|---------|----------|
| `TestNewPoolRejectsUnrepresentableRange` | `NewPool` refuses a `/0` and a `/1` rather than computing a wrapped `size4` (P4) | remove the self-bound → reddens; this is the test that settles P4 by execution rather than by reading |
| `TestAllocateV6RespectsPrefixLongerThan64` | A `/96` pool hands out addresses inside the configured prefix (P6) | current code reddens it today |
| `TestLeaseReleasedOnSATeardown` | An established-then-torn-down session returns its address; `Available()` recovers (P3) | remove the `Release` call → reddens |
| `TestLeaseQuotaPerIdentity` | One identity cannot exceed `maximum-leases-per-identity` (P2) | remove the quota → reddens |

---

## 10. Id allocation

`check_id_allocation` (the retired `scripts/dev/rfc_requirements.py` (current producer: `internal/le/rfc/rfc.go`)) refuses a new id whose
ordinal is at or below its section's high-water mark. The mark comes from the **committed
HEAD** summaries, and **a section with no mark at all is skipped entirely** (`:500-502`).

### Marks measured 2026-07-31

`rfc/short/rfc7296.md` is modified in the working tree and unstaged
(`git status --porcelain` reports a leading space then `M`). **For WP-9's six sections the
working tree and HEAD hold an identical id set**, so the ambiguity that complicated WP-3's
allocation does not arise here: the marks are the same read either way.

| Section | HEAD ids | Mark | Verdict for the Appendix A ordinal |
|---------|----------|------|-------------------------------------|
| `RFC7296-1.7` | `-2` | **2** | **`1.7-1` is REFUSED.** Needs -3 or higher |
| `RFC7296-2.19` | none | **none** | `-1` .. `-6` accepted as written |
| `RFC7296-2.20` | none | **none** | `-1` accepted as written |
| `RFC7296-3.15.1` | none | **none** | `-1` .. `-7` accepted as written |
| `RFC7296-4` | none | **none** | `-2`, `-3` accepted **only if the landing order is right** -- see below |

`_head_of` keys on the section STRING (`rfc_requirements.py`), so `RFC7296-3.15`
and `RFC7296-3.15.1` are distinct scopes and do not share a mark. Appendix A holds no
`RFC7296-3.15-*` row, so §3.15 is not in play.

### §1.7: one renumbering, no competition

`1.7-1` lands as **`RFC7296-1.7-3`**. WP-9 is the sole claimant of §1.7's remaining row
(`1.7-2` is already at HEAD), so nothing can move the mark first. Correct Appendix A in
the same commit, on the precedent set for `1.4-2` → `1.4-5`
(the rfcgate-1b RFC 7296 pilot spec).

### §2.19, §2.20, §3.15.1: safe, WP-9 owns every row

Appendix A holds exactly six §2.19 rows, one §2.20 row and seven §3.15.1 rows, and WP-9
owns all fourteen. No other package claims one, so no other package can move a mark first.
**Land each section as one contiguous block, in ascending ordinal order.** Contiguity is
not required by the checker, but landing a section's rows out of order strands the
ordinals in between, which this spec has already paid for three times
(`:1363-1387`, `:1399-1414`, `:1416-1438`).

### §4: THREE claimants, and the spec's own phase order strands WP-9

This is the §3.10.1 trap from `plan/handover/02-design-wp3.md` section 6, one section
over. Appendix A holds four §4 rows and WP-9 owns only the middle two:

| Appendix A id | Owner | Phase list item |
|---------------|-------|-----------------|
| `4-1` | WP-12 (notify shape, expired SA, INITIAL_CONTACT, NO_ADDITIONAL_SAS) | **7** (`:672`) |
| `4-2` | **WP-9** | **15** (`:723`) |
| `4-3` | **WP-9** | **15** |
| `4-4` | WP-10 (certificates, identities, management interface) | **14** (`:717`) |

The section has no mark, so whichever package lands first sets it. Working the cases:

| Landing order | Result |
|---------------|--------|
| `4-1`, then `4-2`/`4-3`, then `4-4` | mark goes 1 → 3 → 4. **All four land at their Appendix A ordinals.** Correct |
| `4-4` first (WP-10 at item 14, before WP-9 at item 15) | mark jumps to **4**. **`4-2` and `4-3` are REFUSED**, and `4-1` is refused too. Three rows renumbered out of document order |
| `4-2`/`4-3` first | mark goes to 3. `4-1` is REFUSED and must become -5 or higher, out of document order |

**The spec's phase list schedules WP-10 (item 14) before WP-9 (item 15). That ordering
strands WP-9's §4 rows.** This is a real, actionable defect in the plan, not a
hypothetical.

**The rule: §4 must land in ascending ordinal order across packages -- `4-1`, then
`4-2`/`4-3`, then `4-4`.** Three ways to honour it, in preference order:

1. WP-10 defers `4-4` until after WP-9's §4 pair lands. Cheapest: `4-4` is a
   configuration-capability row with no dependency on the other three.
2. All four §4 rows land in one commit.
3. WP-9 lands its §4 pair early, in a small separate commit, before WP-10 runs.

**Recompute at the moment of landing. Never hardcode from this document.** One command
per section:

    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-4-[0-9]*' | sort -V | tail -1

### Summary

| Appendix A id | Lands as | Note |
|---------------|----------|------|
| `RFC7296-1.7-1` | **`RFC7296-1.7-3`** | forced renumber, mark is 2 |
| `RFC7296-2.19-1` .. `-6` | unchanged | sole claimant, land as a block |
| `RFC7296-2.20-1` | unchanged | sole claimant |
| `RFC7296-3.15.1-1` .. `-7` | unchanged | sole claimant, land as a block |
| `RFC7296-4-2`, `-3` | unchanged **only if `4-4` has not landed** | otherwise -5, -6, out of document order |

---

## 11. Is this a feature build? Yes, and here is the honest breakdown

**This package is not a test-writing exercise. Nine of seventeen rows need production
code, and the code they need is a user-facing feature with a config surface, an
authorization model, a resource allocator and an interop obligation.** It is the largest
remaining package for a reason.

The spec anticipated this. Risk R-8 (the rfcgate-1b RFC 7296 pilot spec) says WP-9
"adds a whole operator-facing feature surface ... If it turns out to be a spec-sized
feature in its own right, that is a scope question for Thomas -- raised as a question,
never resolved by dropping the rows."

**Raising it: this is a spec-sized feature.** It should probably be its own spec, with the
17 rows as its acceptance criteria, rather than a phase item inside the compliance-gate
pilot. `plan/spec-ipsec-remote-access.md` is named by `engine/config.go` as the owner
of this surface and may already be that spec. That is owner item OI-5.

The mitigating fact from section 1 is real: the YANG, the config parse, the validation and
the pool all exist. The estimate below reflects joining and fixing rather than building
from nothing.

### Phases

| # | Phase | Work | Rows unblocked | Estimate |
|---|-------|------|----------------|----------|
| A | **Codec correctness** | D1, D2, D3, D4, D6, D7 in `payload_cp.go`, plus codec-layer tests. Independent of everything else, lands first, and D1 is a live `2.5-7` defect | none directly; unblocks `3.15.1-4`, `4-2`, `1.7-1` | 0.5 day |
| B | **Pool hardening** | P2, P4, P5, P6 in `eap/pool.go`; identity-bound allocation; the lease table and expiry (P3). Independent of the CP wire path | none directly; unblocks `4-3` | 1 day |
| C | **Config surface** | `container configuration-payload`; `leaf dns` → `leaf-list dns`; the multi-pool decision; parse, validate, doctor check, completion | `2.19-6`, `2.20-1` | 0.5 day |
| D | **The consumer** | `engine/cp.go`; the CP case at all **three** drop sites; the `startResponderEAP` signature; the reply insertion at `responder.go`; TS narrowing before `:613`; P1 (`_ = ipPool` → real wiring) | `2.19-2`, `2.19-3`, `4-2`, `4-3`, `3.15.1-1` | 1.5 days |
| E | **Authorization and error paths** | The two fail-closed guards (8.1, 8.2); `FAILED_CP_REQUIRED` emission and its pre-`:623` short-circuit; `INTERNAL_ADDRESS_FAILURE` (8.4) | `2.19-5`, `2.19-6` | 0.5 day |
| F | **Tests** | 34 tagged tests, every mutation in 9.3 run and reverted, the pool tests in 9.4, `test/ipsec/ipsec-remote-access-cp.ci`, and the strongSwan scenario `remote-access-cp` | all 17 proven | 1.5 days |
| G | **Discovery and closure** | `docs/features.md`, the guide, `docs/architecture/wire/`, `docs/features/rfc-status.md` rows, the 17 summary rows, `./le rfc index-update`, the R-8 Integration Checklist re-answer | -- | 0.5 day |

**Total: roughly 6 days**, and phases A, B and C are genuinely parallel.

**Phases A and B are worth landing on their own merits even if the rest slips.** A fixes a
live `2.5-7` violation; B fixes a live out-of-range address bug (P6) and removes the
exhaustion-by-churn failure (P3). Neither depends on the CP consumer.

---

## 12. What this must NOT break

| Invariant | Why it is at risk | The guard |
|-----------|-------------------|-----------|
| **`RFC7296-2.5-13`: a message MUST NOT be rejected over payload order** | An implementer enforcing `2.19-2` adds a receive-side order check. `2.19-2` is a **send** obligation only. The comment at `responder.go` says the walk collects and checks afterward, deliberately | `2.19-2`'s tag says "send side only". The existing `2.5-13` pair reddens. **WP-9 adds no receive-side order check of any kind** |
| **`RFC7296-3.15.1-4` tolerance is not turned into rejection** | The same instinct that adds an order check adds "reject unknown attribute". The RFC says ignore | `3.15.1-4`'s positive drives a request carrying four unknown types and asserts the session still completes |
| **`RFC7296-2.5-9` / `-11`: critical-bit handling for the CP payload type** | Type 47 decodes successfully today, so `ErrUnsupportedCrit` (`wire/chain.go`) never fires for it. Adding a consumer must not change that | The existing `2.5-9` and `2.5-11` pairs stay green. WP-9 touches neither parser's critical branch |
| **The IKE SA survives a FAILED_CP_REQUIRED** | The natural implementation of "fail the request" tears the session down | `2.19-6`'s positive asserts `StateEstablished` after the refusal |
| **No Child SA or key material is installed on a refusal** | `createFirstChildSA` runs at `responder.go`, before the payload list | `2.19-6`'s positive asserts no Child SA was installed. The mutation that moves the short-circuit after `:623` must redden it |
| **EAP sessions still establish** | Phase D changes `startResponderEAP`'s signature (`responder_eap.go`) and `handleResponderEAP`'s walk | `TestResponderEAPSessionWired` (`internal/component/ike/engine/responder_test.go`) stays green |
| **Site-to-site peers are unaffected** | `cpPolicyFor` must return a determined `required=false` for a site-to-site profile, not a miss | `2.19-6`'s negative drives a site-to-site peer and asserts normal establishment |
| **Every `test/ipsec/` `.ci` and `test/interop-ipsec/` scenario stays green** | the rfcgate-1b RFC 7296 pilot spec | WP-9 changes no wire byte for a peer that sends no CP: the reply is emitted only when `cpRequestFrom` returns `ok` |
| **`RFC7296-2.5-6` / `-7` (RESERVED)** | D1 and D2 are literally `2.5-7` and `2.5-6` fixes for this payload | Both existing pairs stay green; the CP codec joins the set of producers they range over |

---

## 13. Risks

| Id | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-WP9-1 | **The `2.19-5` guard fails open on `CFGType`.** The idiomatic `if cpReq != nil` hands a leased address to a peer that sent `CP(CFG_SET)` | none: the happy path passes, and the CFG_SET case is never exercised by an ordinary client | Section 8.1's four conditions in **one** function returning explicit `ok`. The `CFGType` mutation in 9.3 is mandatory and must be run separately from the attribute-presence mutation |
| R-WP9-2 | **Leases are never released, and the pool exhausts under normal churn.** `Release` has no non-test caller and no lease concept exists | `Available()` decreases monotonically; the first report is a user who cannot connect | P3 in phase B. `TestLeaseReleasedOnSATeardown`. The doctor check surfaces utilisation before exhaustion is total |
| R-WP9-3 | **Only the direct IKE_AUTH drop site is wired, and the feature silently does not work against real clients.** Road warriors send CFG_REQUEST in the final EAP IKE_AUTH, which is `handleResponderEAP` (`responder_eap.go`) | the unit tests pass; the strongSwan scenario fails or hangs | `4-2`'s negative drives the EAP path specifically, and its mutation reddens ONLY the negative. The anti-vacuity guard in 9.3 requires the sample to contain both flows |
| R-WP9-4 | **The leased address never reaches the traffic selectors.** Allocation placed after `buildChildSAResponsePayloads` (`responder.go`) leaves `sa.NegotiatedTSi` as the client's proposal | the client gets an address and cannot route; interop scenario fails at the traffic stage, not the negotiation stage | Section 7.2 fixes the ordering. The interop scenario must assert traffic flows, not merely that a CFG_REPLY arrived |
| R-WP9-5 | **§4 ids are stranded because WP-10 lands `4-4` first.** The spec's own phase list schedules it that way | `check_id_allocation` fails naming `4-2` | Section 10. Recompute at landing; prefer deferring `4-4` |
| R-WP9-6 | **The R-bit defect (D1) is not fixed, and `3.15.1-4` is proven by a test that never sets the bit.** The row goes green while a conforming peer is misparsed | none; a peer that never sets the bit interoperates fine | `3.15.1-4`'s negative IS the R-bit case, and its mutation is "revert the D1 mask" |
| R-WP9-7 | **`2.19-6` tears down the IKE SA.** "Fail the request" reads as "kill the session" | the client retries in a loop; `rfc/full/rfc7296.txt:3185` is violated | `2.19-6`'s positive asserts `StateEstablished` after the refusal |
| R-WP9-8 | **An authenticated client drains the pool through parallel IKE SAs.** `Allocate()` has no identity and no quota | pool utilisation spikes from one identity | `maximum-leases-per-identity`, default 1, plus address reuse per `rfc/full/rfc7296.txt:6374-6375` |
| R-WP9-9 | **`plan/spec-ipsec-remote-access.md` already holds decisions this design re-litigates.** It is named by `engine/config.go` as the owner of this surface and was NOT read in this pass | the implementer finds a conflicting design mid-phase | **Read it and `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` before phase A.** If it is the real owner, OI-5 answers itself |
| R-WP9-10 | **The interop scenario is tagged and `./le rfc check` refuses it.** `test/interop-ipsec/` is not a carrier | the gate fails naming the file | Section 9.1. Every tag lives in a `_test.go` or in `test/ipsec/*.ci` |
| R-WP9-11 | **P6 hands out addresses outside the configured prefix.** `allocateV6` writes the host ID into `ip6[8:]` only, while the validator permits `/48../126` | a `/96` pool leases addresses in a different subnet | Phase B, `TestAllocateV6RespectsPrefixLongerThan64`. This bug is live today |
| R-WP9-12 | **Engine line numbers move under a concurrent agent.** `internal/component/ike/engine/` is being edited now | a tag cites a line holding different code | Every citation here names its function. Re-locate by function name before quoting a line |

---

## 14. Items for the owner

Each is a place where something narrower than full compliance plus full proof is on the
table, which `ai/rules/rfc-compliance.md` forbids me from choosing. **None is a request to
skip anything.** In every case the tagged pair gets written and the row stays gated; the
question is which way the obligation is discharged.

**OI-1 -- the IRAC role (`RFC7296-2.19-1`, `2.19-4`).**
§4 says "Implementations are not required to support requesting temporary IP addresses"
(`rfc/full/rfc7296.txt:6860-6861`), so declining the client role is RFC-sanctioned. Ze
declines it today by having no CP producer at all in `buildAuthRequest`
(`internal/component/ike/engine/auth.go`). **Cost of implementing it:** a CP
producer in `buildAuthRequest`, an initiator-side CFG_REPLY consumer, and a way for the
operator to say "this peer needs a virtual IP" -- roughly phase D again, on the initiator
side, about 1.5 days. Ask: "do you want Ze to be an IRAC as well as an IRAS, or is
declining the client role the recorded answer?"

**OI-2 -- ignoring CFG_SET (`RFC7296-3.15.1-5`, `-6`, `-7`).**
"An implementation of this specification MAY ignore CFG_SET payloads"
(`rfc/full/rfc7296.txt:6477`), and the RFC adds "There are currently no defined uses for
the CFG_SET/CFG_ACK exchange" (`:6474-6475`). Exercising an explicit MAY is not a
deviation. **Cost of implementing CFG_SET/CFG_ACK instead:** an attribute-acceptance model
Ze has no use for, plus the CFG_ACK builder -- perhaps 0.5 day, for an exchange with no
defined use. Ask: "confirm that ignoring CFG_SET is the recorded answer, so the three rows
are discharged by that choice rather than by absence."

**OI-3 -- answering SUPPORTED_ATTRIBUTES (`RFC7296-3.15.1-3`).**
The row binds the sender of a request, which Ze is not. Whether Ze ANSWERS the query as
responder is open. **Answering is strictly more compliance and needs no permission**
(`ai/rules/completion.md`); it costs about half a day, given the constant (type 14) does not
exist yet. Ask only if you intend to decline.

**OI-4 -- §3.15.4 has no rows (`INTERNAL_ADDRESS_FAILURE`).**
`rfc/full/rfc7296.txt:6652-6656` requires the notification on allocation failure, and
Appendix A extracted no §3.15.4 row. `ai/rules/rfc-compliance.md` is explicit that "an
unextracted obligation is still an obligation" and that the gate's silence is not
conformance. WP-9 implements it either way. Ask: "should §3.15.4 gain checklist rows in
this package, or is that the extraction sign-off's business?"

**OI-5 -- is this a spec of its own?**
Section 11 and R-8 (the rfcgate-1b RFC 7296 pilot spec). Six days, a new operator
config surface, an authorization model and a resource allocator inside what is otherwise a
compliance-gate pilot. `plan/spec-ipsec-remote-access.md` may already own it. Ask: "does
WP-9 stay a phase of the pilot, or become its own spec with these 17 rows as acceptance
criteria?"

---

## 15. Summary row texts to add

Land them in section order in the checklist of `rfc/short/rfc7296.md`. Ordinals assume the
§4 landing order in section 10; **recompute every one at the moment of landing**.

Each row is ONE physical line in the file -- `parse_checklist_line` reads one row per line.
The wrapping below is for this document only. The parser validates that the id's section
segment agrees with the `(§X.Y)` citation, so the citations are load-bearing, not
decoration.

    - [ ] [RFC7296-1.7-3] [MUST] Implementations that conform to this document MUST ignore
      proposals that have configuration attribute type 5, the old value for
      INTERNAL_ADDRESS_EXPIRY (§1.7)

    - [ ] [RFC7296-2.19-1] [MUST] Since the IKE_AUTH exchange creates an IKE SA and a Child
      SA, the IRAC MUST request the IRAS-controlled address (and optionally other
      information concerning the protected network) in the IKE_AUTH exchange (§2.19, §4)

    - [ ] [RFC7296-2.19-2] [MUST] In all cases, the CP payload MUST be inserted before the
      SA payload (§2.19)

    - [ ] [RFC7296-2.19-3] [MUST] In variations of the protocol where there are multiple
      IKE_AUTH exchanges, the CP payloads MUST be inserted in the messages containing the
      SA payloads (§2.19)

    - [ ] [RFC7296-2.19-4] [MUST] CP(CFG_REQUEST) MUST contain at least an INTERNAL_ADDRESS
      attribute (either IPv4 or IPv6) but MAY contain any number of additional attributes
      the initiator wants returned in the response (§2.19)

    - [ ] [RFC7296-2.19-5] [MUST NOT] The responder MUST NOT send a CFG_REPLY without
      having first received a CP(CFG_REQUEST) from the initiator, because we do not want
      the IRAS to perform an unnecessary configuration lookup if the IRAC cannot process
      the REPLY (§2.19)

    - [ ] [RFC7296-2.19-6] [MUST] In the case where the IRAS's configuration requires that
      CP be used for a given identity IDi, but IRAC has failed to send a CP(CFG_REQUEST),
      IRAS MUST fail the request, and terminate the Child SA creation with a
      FAILED_CP_REQUIRED error (§2.19)

    - [ ] [RFC7296-2.20-1] [MUST] An IKE implementation MAY decline to give out version
      information prior to authentication or even after authentication in case some
      implementation is known to have some security weakness; in that case, it MUST either
      return an empty string or no CP payload if CP is not supported (§2.20)

    - [ ] [RFC7296-3.15.1-1] [MUST] Only one netmask is allowed in the request and response
      messages, and it MUST be used only with an INTERNAL_IP4_ADDRESS attribute (§3.15.1)

    - [ ] [RFC7296-3.15.1-2] [MUST NOT] Non-empty values for the INTERNAL_IP4_NETMASK
      attribute in a CFG_REQUEST do not make sense and thus MUST NOT be included (§3.15.1)

    - [ ] [RFC7296-3.15.1-3] [MUST] When used within a Request, the SUPPORTED_ATTRIBUTES
      attribute MUST be zero-length and specifies a query to the responder to reply back
      with all of the attributes that it supports (§3.15.1)

    - [ ] [RFC7296-3.15.1-4] [MUST] Unrecognized or unsupported attributes MUST be ignored
      in both requests and responses (§3.15.1)

    - [ ] [RFC7296-3.15.1-5] [MUST] The responder MUST return a Configuration payload if it
      accepted any of the configuration data, and the Configuration payload MUST contain
      the attributes that the responder accepted with zero-length data (§3.15.1)

    - [ ] [RFC7296-3.15.1-6] [MUST NOT] Those attributes that it did not accept MUST NOT be
      in the CFG_ACK Configuration payload (§3.15.1)

    - [ ] [RFC7296-3.15.1-7] [MUST] If no attributes were accepted, the responder MUST
      return either an empty CFG_ACK payload or a response message without a CFG_ACK
      payload (§3.15.1)

    - [ ] [RFC7296-4-2] [MUST] If an implementation supports responding to such requests,
      it MUST parse the CP payload of type CFG_REQUEST in the first message in the IKE_AUTH
      exchange and recognize a field of type INTERNAL_IP4_ADDRESS or INTERNAL_IP6_ADDRESS
      (§4)

    - [ ] [RFC7296-4-3] [MUST] If it supports leasing an address of the appropriate type,
      it MUST return a CP payload of type CFG_REPLY containing an address of the requested
      type (§4)

**Two row-text corrections against Appendix A**, both restoring elided material rather than
changing an obligation:

- **`2.19-4`** -- Appendix A drops "but MAY contain any number of additional attributes the
  initiator wants returned in the response". The drop does not change the MUST, but the
  clause is what makes clear the attribute list is a floor and not a ceiling. Restore it.
- **`2.20-1`** -- Appendix A quotes only "In that case, it MUST either return an empty
  string or no CP payload if CP is not supported", leaving "that case" unresolvable.
  Restore enough of `rfc/full/rfc7296.txt:3206-3208` to name the antecedent.

After the rows land, run `./le rfc index-update` and commit `ai/RFC-REQUIREMENTS.md` **in the
same commit**. The ledger records each tagged test's `file:line`, and both verify modes of
`./le rfc check` fail on a stale ledger (`ai/rules/testing.md`, RFC-Tagged Tests). With 34
new tags across four files, this is not optional bookkeeping -- a skipped regen lands on
the next session as a cross-commit diff.

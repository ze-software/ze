<!-- ste: ignore-file preserved verbatim from the design pass. It quotes RFC 7296 at length and carries producer citations for header flags and version negotiation. An edit for prose style risks altering a technical claim the next session relies on, and the document is a working record rather than published prose. -->

# WP-5 design -- header flags, version negotiation, critical bit

Rows: `RFC7296-2.5-5`, `RFC7296-2.5-12`, `RFC7296-3.2-1`, `RFC7296-3.12-1`.
Source spec: `plan/spec-rfcgate-1b-rfc7296-pilot.md`, phase list item 6 (`:641-644`).

**Read-only design.** No tracked file was modified. Every `file:line` below was read in
the working tree on 2026-07-31. A second agent is editing `internal/component/ike/engine/`
for WP-1, so engine line numbers can move. Every citation names the FUNCTION as well as the
line, and the implementer must re-locate by function name.

---

## 0. Verdict

| Row | Appendix A class | This design's verdict | Needs code? |
|-----|------------------|-----------------------|-------------|
| `RFC7296-2.5-5` | **NOT IMPL** | **conformant.** The consequent is unimplementable, and the antecedent is unreachable | no |
| `RFC7296-2.5-12` | **NOT IMPL** | **conformant.** Three generic-header producers, none of which can emit a set critical bit | no |
| `RFC7296-3.2-1` | **NOT IMPL** | **conformant.** Same three producers | no |
| `RFC7296-3.12-1` | **NOT IMPL** | **conformant.** Ze sends no Vendor ID payload, and a received one changes no decision | no |

**All four are conformant today. WP-5 writes eight tagged tests and adds no production
code.** This contradicts Appendix A, which classes all four `NOT IMPL`, and it contradicts
the 2026-07-30 triage, which says three of them "hold only by absence" and "MUST stay NOT
IMPL" (`plan/spec-rfcgate-1b-rfc7296-pilot.md:1581-1583`). Section 2 gives the reading that
replaces the triage's, and section 7 gives the one item the owner must confirm.

**The package renumbers three of the four ids.** `3.2-1` and `3.12-1` are both below their
sections' high-water marks and `check_id_allocation` refuses both. See section 5.

---

## 1. `RFC7296-2.5-5` -- version downgrade recovery

### The obligation, verbatim

> "If they mistakenly (perhaps through an active attacker sending error messages)
> negotiate to version n, then both will notice that the other side can support a higher
> version number, and they MUST break the connection and reconnect using version n+1."

`rfc/full/rfc7296.txt:1670-1673`. The MUST is on `:1673`.

Appendix A drops the parenthetical "(perhaps through an active attacker sending error
messages)". The drop does not widen or narrow the obligation, so it is a legitimate
elision. **Restore it anyway.** The elided clause names the attack, and the row's own text
should carry the reason the singleton version set is a defence.

### The setup sentence the obligation depends on

> "If the initiator is capable of speaking versions n, n+1, and n+2, and the responder is
> capable of speaking versions n and n+1, then they will negotiate speaking n+1, where the
> initiator will set a flag indicating its ability to speak a higher version."

`rfc/full/rfc7296.txt:1666-1670`. This is indicative, not normative. It states the
antecedent: an implementation with a version set of size two or more, and a flag by which
each side announces a higher capability.

### The flag the antecedent needs

> "V (Version) - This bit indicates that the transmitter is capable of speaking a higher
> major version number of the protocol than the one indicated in the major version number
> field. Implementations of IKEv2 MUST clear this bit when sending and MUST ignore it in
> incoming messages."

`rfc/full/rfc7296.txt:4129-4133`. That is `RFC7296-3.1-11`, which is already gated with a
positive and two negatives.

### What Ze does today

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| Every message Ze sends carries major version 2 | `buildSAInitRequest` | `internal/component/ike/engine/initiator.go:75-76` |
| Same | `buildSAInitResponse` | `internal/component/ike/engine/responder.go:259-260` |
| Same | `sendSAInitNotify` | `internal/component/ike/engine/responder.go:337` |
| Same, for every encrypted exchange | `writeAuthHeaderWithMsgID` | `internal/component/ike/engine/auth.go:755-766` (`MajorVersion: 2` at `:759`) |
| Port 500 drops every major version that is not 2 | `dispatchInbound` | `internal/component/ike/engine/register.go:645` (`if pkt.Data[17]>>4 != 2 { continue }`) |
| Port 4500 drops every major version that is not 2 | `dispatchNATTInbound` | `internal/component/ike/engine/register.go:466` (`if ikeData[17]>>4 != 2 { continue }`) |
| No sender sets the V bit | none: `wire.FlagVersion` has no non-test referent | `internal/component/ike/wire/header.go:21` is the only declaration |
| No receiver reads the V bit | `Header.ReadFrom` stores the flags octet and every consumer tests only `FlagResponse` or `FlagInitiator` | `internal/component/ike/wire/header.go:63` |

**Note the stale locator.** OR-D cites `register.go:455` and `register.go:625` for these two
equality tests (`plan/spec-rfcgate-1b-rfc7296-pilot.md:1643-1644`). Neither line holds the
test today. `:455` is `if transport.IsNATKeepalive(...)` and `:625` is `ps.setPendingSA(sa)`.
The tests are at `:466` and `:645`. The existing header test's comments are stale too:
`internal/component/ike/engine/rfc7296_header_test.go:88` and `:117` cite `register.go:637`
and `:635-638`. Fix the comments in the same commit.

### Is it conformant?

**Yes, and for two independent reasons.**

**The consequent is unimplementable.** The obligation orders a reconnect "using version
n+1". Ze's only major version is 2, so n+1 is 3. RFC 7296 defines version 2.0 and no IKEv3
exists. The action the MUST names has no target. This is a fact about the protocol family,
not a scope reduction.

**The antecedent is unreachable.** Ze's supported major-version set is the singleton {2},
enforced at both dispatch loops. Ze never announces a higher capability, because it never
sets the V bit. Ze never observes a peer's announcement, because it ignores the V bit and
drops any datagram whose major version is not 2. So the state "both notice that the other
side can support a higher version number" cannot arise on Ze's side of any session.

Every clause of that argument rests on a property the code HAS. None rests on the absence
of a guard, so the argument does not expire when the next guard lands
(`plan/spec-rfcgate-1b-rfc7296-pilot.md:1843-1863`).

### The tagged pair

New file `internal/component/ike/engine/rfc7296_version_test.go`, or an addition to the
existing `internal/component/ike/engine/rfc7296_header_test.go`. **Prefer the existing
file.** It already owns the version and flag surface, it holds the `engineBuiltMessages`
harness, and a second file would split one observable across two homes.

**Positive -- `TestSupportedMajorVersionSetIsSingleton`.**
Sweep the full major nibble, 0 through 15, against `dispatchInbound`. Assert that only
major version 2 is delivered to `ps.inbound`. Reuse the ordering trick of
`TestHigherMajorVersionDropped` (`rfc7296_header_test.go:172-198`): send every rejected
version first, then the version-2 datagram last, and assert exactly one delivery. UDP on
loopback keeps order and `dispatchInbound` is one goroutine, so a single delivery proves
the other fifteen were dropped rather than delayed. Then assert over
`engineBuiltMessages(t)` that every built message carries major version 2, which closes the
send side.

The tag states the argument: Ze's version set is {2}, a singleton has no pair (n, n+1) both
supported, so the mistaken-downgrade state the MUST governs is unreachable; and version 3
does not exist, so the reconnect the MUST orders has no target.

**Negative -- `TestNATTDispatchAppliesTheSameVersionGate`.**
Drive `dispatchNATTInbound` with the same sweep. Prefix each datagram with the four-zero
non-ESP marker (`transport.StripNonESPMarker`, `internal/component/ike/transport/nat.go:67-81`).
Assert only major version 2 is delivered.

This is the discriminating half, and it is genuinely new coverage. `RFC7296-2.5-2`'s
existing pair drives `dispatchInbound` only (`rfc7296_header_test.go:122-199`), so the
port-4500 producer of the same obligation is unproven today. That is the second-producer
shape the spec calls the most dangerous it has found
(`plan/spec-rfcgate-1b-rfc7296-pilot.md:1778-1783`). The negative's argument: the singleton
claim is a claim about EVERY entry point, and one entry point accepting a second version
would make the antecedent reachable while no recovery code exists.

### Mutations

| Mutation | Site | Must redden |
|----------|------|-------------|
| `!= 2` becomes `> 2` (version 1 is now accepted) | `dispatchInbound`, `register.go:645` | the positive |
| The version test is deleted | `dispatchInbound`, `register.go:645` | the positive |
| `MajorVersion: 2` becomes `3` | `writeAuthHeaderWithMsgID`, `auth.go:759` | the positive's built-message half |
| `!= 2` becomes `> 2` | `dispatchNATTInbound`, `register.go:466` | the negative ONLY. `TestHigherMajorVersionDropped` stays green, which is what proves the negative gates something nothing else does |

### Does it overlap `RFC7296-2.5-2`?

`2.5-2` is "a HIGHER major version MUST be dropped". `2.5-5` is "a mistaken downgrade MUST
be repaired". They share one producing fact and they are different obligations. The spec's
own lesson applies: when two rows govern one observable, the test must establish which case
it is in (`plan/spec-rfcgate-1b-rfc7296-pilot.md:1839-1841`). The tag for `2.5-5` must
therefore say that the sweep covers versions BELOW 2 as well as above, because it is the
rejection of version 1 that makes the set a singleton, and `2.5-2` says nothing about
version 1.

---

## 2. `RFC7296-2.5-12`, `RFC7296-3.2-1`, `RFC7296-3.12-1` -- the critical bit

The three rows share one send-side surface and differ by scope. Section 2.4 keeps them
apart.

### 2.1 The obligations, verbatim

**`RFC7296-2.5-12`** (MUST NOT):

> "Payloads sent in IKE response messages MUST NOT have the critical flag set."

`rfc/full/rfc7296.txt:1711-1712`. Appendix A quotes this EXACTLY. No change needed.

**`RFC7296-3.2-1`** (MUST), from the Critical bit bullet of the generic payload header:

> "MUST be set to zero for payload types defined in this document."

`rfc/full/rfc7296.txt:4237-4238`. Appendix A restores the bullet's subject, "The Critical
bit". That is a subject restoration, not an overstatement. Keep it.

**`RFC7296-3.12-1`** (MUST NOT):

> "A Vendor ID payload MUST NOT change the interpretation of any information defined in
> this specification (i.e., the critical bit MUST be set to 0)."

`rfc/full/rfc7296.txt:5852-5853`. Appendix A writes ", i.e., the critical bit MUST be set to
0" where the source has "(i.e., the critical bit MUST be set to 0)". Restore the
parentheses.

### 2.2 What Ze does today: three producers, and only one of them can express the bit

Every message Ze sends has its generic payload headers written by exactly one of three
functions.

| # | Producer | `file:line` | Covers | Can it emit a set critical bit? |
|---|----------|-------------|--------|----------------------------------|
| 1 | `Message.WriteTo` | `internal/component/ike/wire/message.go:30` (`gh.Critical = m.Payloads[i].Critical`) | the outer chain of a plaintext message: IKE_SA_INIT request and response, and the IKE_SA_INIT notify | only if a caller sets `PayloadEntry.Critical` |
| 2 | `buildEncryptedMessageEx` | `internal/component/ike/engine/auth.go:194` (`gh.Critical = innerPayloads[i].Critical`) | the inner chain inside every SK payload | only if a caller sets `PayloadEntry.Critical` |
| 3 | `writeAuthHeaderWithMsgID` | `internal/component/ike/engine/auth.go:767-771` | the SK payload's OWN generic header on every encrypted message | **no.** The `wire.GenericHeader` literal names `NextPayload` and `Length` only, and nothing assigns `Critical` |

The bit itself is written by `GenericHeader.WriteTo`
(`internal/component/ike/wire/payload.go:73-82`): octet 1 is `0x80` when `Critical` is true
and `0x00` otherwise.

**No caller of producer 1 or producer 2 sets `Critical`.** `grep -rn "Critical"
internal/component/ike/ --include=*.go` outside tests returns eleven lines, and every
`wire.PayloadEntry` literal in the tree omits the field. The two sites that DO set it are
both on the PARSE path: `Message.ReadFrom` at `wire/message.go:139` and `ParsePayloadChain`
at `wire/chain.go:45`, each copying `gh.Critical` off the wire.

**No response builder echoes a parsed payload.** Ten sites build a message carrying
`wire.FlagResponse`:

`responder.go:262`, `responder.go:339`, `responder.go:640`, `responder.go:694`,
`responder_eap.go:173`, `responder_eap.go:264`, `rekey.go:207`, `rekey.go:521`,
`rekey.go:597`, and `inbound.go:321`.

Each one constructs fresh `wire.PayloadEntry` literals. `buildSAInitResponse`
(`responder.go:242-247`) and `handleAuthRequest`'s responder half (`responder.go:628-638`)
are the two with several payloads, and both build every payload from negotiated state.
`handleInformationalOwned` passes `nil` inner payloads (`inbound.go:321`).

### 2.3 Are they conformant?

**Yes, all three.**

- `3.2-1`: every payload Ze sends has a type in 33..48, all defined in this document, and
  every one carries a clear critical bit.
- `2.5-12`: every payload in every response Ze sends carries a clear critical bit, at both
  nesting levels.
- `3.12-1`: Ze constructs no Vendor ID payload, so it sends none with the bit set; and a
  received Vendor ID changes no decision, because `PayloadVendorID` has no consumer outside
  the codec (`internal/component/ike/wire/payload_vendor.go`) and its registration
  (`internal/component/ike/wire/payload.go:139-140`).

### 2.4 Why the triage's "expiring negative" verdict does not apply

The triage says these three "hold only by absence" and that a test would take the shape
that expired for `RFC7296-3.4-1` (`plan/spec-rfcgate-1b-rfc7296-pilot.md:1581-1583`). That
reading is too pessimistic, and the counter-example sits in the file WP-5 will edit.

`RFC7296-3.1-11` is the same kind of obligation: "MUST clear the V bit when sending". Its
pair is `TestBuiltMessagesClearXAndVBits` (`rfc7296_header_test.go:340-371`). The positive
asserts every built message has the bit clear. The negative asserts the ENCODER CAN SET IT,
so the clear is a decision rather than an encoder limitation
(`rfc7296_header_test.go:364-370`). Neither half argues from an absent guard.

The critical bit has the same structure and one more property in its favour: a `true` value
is not merely expressible, it is REACHED from attacker-controlled input on every inbound
message, because both parsers set `PayloadEntry.Critical` from the wire. The wire tests
already prove the round trip: `internal/component/ike/wire/rfc7296_test.go:256` asserts a
parsed payload's `Critical` is true after an encode-decode cycle.

`RFC7296-3.4-1`'s negative expired because it argued "nothing on the receive side
compensates". The negatives below argue "the encoder honours the field, and the parser
produces a true value from real input". Both are properties the code HAS.

### 2.5 The tagged tests

Three rows, one harness, three distinct assertions. Put them in
`internal/component/ike/engine/rfc7296_critical_bit_test.go` (new). The engine package is
the right layer: producers 2 and 3 live there, `engineBuiltMessages` lives there, and the
inner chain is only reachable after decryption with SA keys.

**Harness.** Extend `engineBuiltMessages` (`rfc7296_header_test.go:28-77`) to return, for
each message, the raw bytes plus the decrypted inner chain where one exists. Use
`decryptAndParse` (`internal/component/ike/engine/inbound.go:103-118`) from the peer SA's
point of view: an initiator SA decrypts a responder-built message, because `skRecvEncKey`
selects `SK_er` for an initiator (`auth.go:654-659`). `decryptOneNonce`
(`role_direction_test.go:42-72`) is the existing worked example of the same call sequence.

**Anti-vacuity guard, mandatory.** `handleInformationalOwned` builds its response with a nil
inner chain, so that message contributes zero inner payloads. The test MUST assert that the
sample contains at least one response with a non-empty OUTER chain (the IKE_SA_INIT
response, four or more payloads) and at least one response with a non-empty INNER chain (the
IKE_AUTH response: ID, AUTH, SA, TSi, TSr). Without that guard the whole assertion can pass
over an empty set.

| Row | Test | Positive asserts | Negative asserts |
|-----|------|------------------|------------------|
| `3.2-1` | `TestDefinedPayloadTypesAreSentUncritical` | over EVERY built message, request and response, every payload whose type is in the defined set 33..48 has a clear critical bit. Assert the observed type set is non-empty and name it in the failure message | the encoder honours the field. A `wire.Message` built with `Critical: true` on a defined type produces `0x80` in octet 1, and `Message.ReadFrom` recovers `Critical == true`. The zero on every built message is therefore a builder decision |
| `2.5-12` | `TestResponsePayloadsAreNeverCritical` | over every RESPONSE only, at BOTH nesting levels, every payload has a clear critical bit, whatever its type. Include the SK payload's own generic header, which producer 3 writes | the scope is responses, and the sample distinguishes them. Assert the sample holds at least one request and at least one response, so "every response" is not "every message that happens to exist". Then assert `buildEncryptedMessageEx` DOES propagate a set bit when a caller asks, so the clear bit in the nine response builders is their choice |
| `3.12-1` | `TestVendorIDDoesNotChangeInterpretation` | a Vendor ID payload inserted into a real IKE_SA_INIT request changes no interpretation of the spec-defined payloads. Run `handleSAInitRequest` twice, once with the Vendor ID and once without, and assert the same responder state, the same chosen proposal, and the same response payload TYPE sequence. Do not compare raw bytes: the nonce and the DH public value differ per run | the differential is sensitive. A payload that IS interpreted, for example a second SA payload or a KE payload for a different group, DOES change the outcome. So "no change" is a fact about the Vendor ID, not about a comparison that can never fail |

`3.12-1`'s critical-bit clause is covered by `3.2-1`'s and `2.5-12`'s assertions, which
range over every payload type. If a Vendor ID producer is ever added, those two tests see it.
State that in `3.12-1`'s tag rather than repeating the assertion.

**Keep `3.12-1` apart from `3.12-2`.** `3.12-2` ("Unfamiliar Vendor IDs MUST be ignored") is
proven at the WIRE layer over the payload's octets
(`internal/component/ike/wire/rfc7296_test.go:766-778`). `3.12-1` is proven at the ENGINE
layer over the negotiation outcome. Different layer, different observable, no duplication.

### 2.6 Mutations

| Mutation | Site | Must redden |
|----------|------|-------------|
| `{Payload: &wire.PayloadSA{...}}` becomes `{Payload: ..., Critical: true}` | `buildSAInitResponse`, `responder.go:243` | `3.2-1` positive AND `2.5-12` positive |
| `{Payload: authPayload}` becomes `{Payload: authPayload, Critical: true}` | responder IKE_AUTH, `responder.go:634` | `2.5-12` positive (inner chain) |
| `gh.Critical = m.Payloads[i].Critical` becomes `gh.Critical = true` | `Message.WriteTo`, `wire/message.go:30` | `3.2-1` and `2.5-12` positives. This mutation is broad and will redden much else; the two above are the surgical ones |
| `gh.Critical = innerPayloads[i].Critical` becomes `gh.Critical = true` | `buildEncryptedMessageEx`, `auth.go:194` | `2.5-12` positive (inner chain). Confirms the harness reaches producer 2 |
| `skGH := wire.GenericHeader{...}` gains `Critical: true` | `writeAuthHeaderWithMsgID`, `auth.go:767-771` | `2.5-12` positive (outer SK header). Confirms the harness reaches producer 3 |
| `gh.Critical = m.Payloads[i].Critical` becomes `gh.Critical = false` | `wire/message.go:30` | `3.2-1` negative and `2.5-12` negative. The encoder stops honouring the field, so "the clear bit is a choice" is no longer provable |
| `handleSAInitRequest` returns early when a `*wire.PayloadVendorID` is present | `responder.go`, in the payload loop | `3.12-1` positive |
| The differential runs the same input twice | the `3.12-1` test's own fixture | `3.12-1` negative |

**Run the last two producer mutations explicitly.** They are the only proof that the
extended harness reaches all three generic-header producers. A harness that reaches only
producer 1 would leave the SK header and the inner chain unproven, which is exactly the
second-producer failure this spec has already recorded twice.

### 2.7 A structural assertion worth its 40 lines

The behavioural tests cover the messages Ze builds TODAY. A future builder that sets the
critical bit and is not in `engineBuiltMessages` would pass them.

Add one source-level assertion beside the behavioural pair: walk the non-test Go files under
`internal/component/ike/engine/` with `go/ast` and fail when a `wire.PayloadEntry` composite
literal carries a `Critical` field, or when any expression assigns to a `.Critical` selector.
The precedent is in the tree: `internal/component/l2tp/subsystem_stop_order_test.go`,
`internal/component/bgp/cli/dispatch_parity_test.go` and three sibling `dispatch_parity`
tests all parse Go source inside a unit test.

Scope it to `engine/` only. The two legitimate writers are both in `wire/`
(`message.go:139`, `chain.go:45`) and both are parse-path code, so the rule needs no
allowlist. Tag the assertion under `3.2-1` positive, as the half that covers a producer the
message harness does not enumerate.

This is a recommendation, not a requirement. Drop it if the reviewer judges it
gold-plating. The argument for keeping it is that the spec has now recorded the
second-producer shape three times, and these three rows have exactly that topology.

---

## 3. Code changes

**None.** WP-5 adds tests, four summary rows and tag-comment corrections. It touches no
production file.

The spec's phase list names `engine/fsm.go`, `engine/responder.go` and `wire/payload.go` as
this package's files (`plan/spec-rfcgate-1b-rfc7296-pilot.md:643`). That estimate was
written before the rows were mapped to producers. None of the three needs an edit.

The only non-test edits are comment corrections in
`internal/component/ike/engine/rfc7296_header_test.go` at `:88` and `:117`, which cite
`register.go:637` and `:635-638` for a check that is at `:645`. **Those comments sit inside
`RFC requirement:` tagged blocks**, so `.claude/hooks/pretool-writeedit.py`'s
`rfc-tagged-test` check applies. A comment-only edit passes it: the hook compares the
enclosing function's BEHAVIOUR, and a comment change alters none. Do not touch the
assertions.

---

## 4. What this must NOT break

| Invariant | Why it is at risk | The guard |
|-----------|-------------------|-----------|
| **A recipient MUST IGNORE the critical bit for a payload type it understands** (`RFC7296-3.2-2`, `rfc/full/rfc7296.txt:4236-4237`) | An implementer reading `3.2-1` as a security rule adds a receive-side rejection of a defined type carrying a set bit. That is a VIOLATION, not a hardening | `TestCriticalBitIgnoredForKnownType` (`wire/rfc7296_test.go:420`) and `TestInnerChainIgnoresCriticalOnKnownType` (`wire/rfc7296_innerchain_test.go:179`) both redden. **WP-5 adds no receive-side check of any kind.** |
| `RFC7296-2.5-9`'s two producers stay untouched | `2.5-9` is proven at `wire/message.go:127-129` and `wire/chain.go:38-40`, each with its own mutation (`plan/spec-rfcgate-1b-rfc7296-pilot.md:1396-1397`) | WP-5 reads `PayloadEntry.Critical` on the SEND side only. It changes neither parser |
| `RFC7296-2.5-11` (an unset critical bit on an unsupported type MUST be ignored) | Same parser, same branch | unchanged |
| `RFC7296-3.1-11` (the V bit) | `2.5-5`'s argument CITES `3.1-11`. If a later change makes Ze set or read the V bit, `2.5-5`'s antecedent becomes reachable | `TestBuiltMessagesClearXAndVBits` and `TestXBitsIgnoredOnReceipt` already gate it. `2.5-5`'s tag must name the dependency so a future reader sees the coupling |
| `RFC7296-2.5-2` and `RFC7296-3.1-5` | `2.5-5` extends the same two dispatch guards | The existing pairs stay green. The sweep is additive |
| The nine response builders keep building fresh payloads | A future optimisation that echoes a parsed `PayloadEntry` into a response would carry the peer's critical bit straight back out, violating `2.5-12` | `2.5-12`'s positive reddens, because the harness decrypts and inspects the inner chain |
| Every `test/ipsec-interop/` scenario and every `test/ipsec/` `.ci` stays green | `plan/spec-rfcgate-1b-rfc7296-pilot.md:204-206` | WP-5 changes no wire byte, so no interop behaviour moves |

---

## 5. Id allocation

`check_id_allocation` (`scripts/dev/rfc_requirements.py:477`, refusal at `:503`) refuses a
new id whose ordinal is at or below its section's high-water mark. The mark is computed from
the COMMITTED HEAD summaries by `_git_baseline_ids` (`:1280`), which runs
`git show HEAD:<path>` -- not the index and not the working tree. A section with no mark is
skipped entirely (`:500-502`).

### Marks measured 2026-07-31

`rfc/short/rfc7296.md` is STAGED (`git status --porcelain` reports `M ` with a blank second
column). The staged diff adds no row in any of the three sections, so the marks are the same
whether they are read from HEAD or from the pending commit.

| Section | HEAD ids | Mark | Verdict for the Appendix A ordinal |
|---------|----------|------|-------------------------------------|
| `RFC7296-2.5` | -1, -2, -6, -7, -8, -9, -11, -13 | **13** | `2.5-5` and `2.5-12` are both REFUSED |
| `RFC7296-3.2` | -2, -3 | **3** | **`3.2-1` is REFUSED.** Needs -4 or higher |
| `RFC7296-3.12` | -2, -3 | **3** | **`3.12-1` is REFUSED.** Needs -4 or higher |

`_head_of` keys on the section STRING, so `RFC7296-3.2` and `RFC7296-3.12` are distinct
scopes (`scripts/dev/rfc_requirements.py:454-457`). They do not share a mark.

### §3.2 and §3.12: settled, and WP-5 is the sole claimant

Appendix A holds exactly three rows in each section, and the two unallocated ones are both
WP-5's. No other package claims either section, so nothing can move either mark first.

| Appendix A id | Lands as | Citation |
|---------------|----------|----------|
| `RFC7296-3.2-1` | **`RFC7296-3.2-4`** | `(§3.2)` |
| `RFC7296-3.12-1` | **`RFC7296-3.12-4`** | `(§3.12)` |

Correct Appendix A in the same commit, per the precedent set for `1.4-2` to `1.4-5`
(`plan/spec-rfcgate-1b-rfc7296-pilot.md:1399-1414`) and for `3.10-1`/`3.10-2` to
`3.10-4`/`3.10-5` (`:1416-1427`).

### §2.5: five rows compete, and the rule is "never leave a hole"

Five Appendix A rows in §2.5 are unallocated, and every one is below the mark of 13:

| Appendix A id | Owner | Status |
|---------------|-------|--------|
| `2.5-3` | phase 3, owner ruling OR-D | pending |
| `2.5-4` | phase 2b (one of the eleven promoted rows, `:1625-1627`) | pending |
| `2.5-5` | **WP-5** | this package |
| `2.5-10` | WP-3 (`tmp/design-wp3.md:757`) | pending |
| `2.5-12` | **WP-5** | this package |

`tmp/design-wp3.md:774-778` proposes landing all five together as `2.5-14` through
`2.5-18`, in Appendix A ordinal order, which gives WP-5 `2.5-16` and `2.5-18`.

**That mapping is correct AND safe only if the five land in ONE commit.** It is not a
conflict; it is a precondition. `check_id_allocation` does not require contiguity, so if
WP-5 landed alone at -16 and -18 the mark would jump to 18 and ordinals 14, 15 and 17 would
be **permanently unusable** by WP-3 and phase 2b. Three ids would be stranded, for nothing.

**The rule: whichever package lands first in §2.5 takes the contiguous block starting at
mark+1, in Appendix A ordinal order within that block.**

| Landing order | `2.5-5` becomes | `2.5-12` becomes |
|---------------|-----------------|------------------|
| All five together, WP-3's mapping | `2.5-16` | `2.5-18` |
| WP-5 lands first, alone | **`2.5-14`** | **`2.5-15`** |
| WP-3 lands first alone (taking -14), then WP-5 | `2.5-15` | `2.5-16` |

**Recompute at the moment of landing. Never hardcode.** The check is one command:

    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-2\.5-[0-9]*' | sort -V | tail -1

WP-5 is scheduled before WP-3 in the spec's phase list (item 6 versus item 8), but the
2026-07-30 re-triage renumbered the packages and put the notify package first
(`plan/spec-rfcgate-1b-rfc7296-pilot.md:1540-1560`). So the landing order is genuinely
open, and the design must not assume it.

### A naming collision the implementer will meet

The 2026-07-30 re-triage RENUMBERED the work packages. "WP-5" now names "Unprotected
messages, and messages that match no SA" (9 rows), and this package's four rows sit in the
new "WP-6, Header flags, payload flags, INFORMATIONAL protection" (5 rows)
(`plan/spec-rfcgate-1b-rfc7296-pilot.md:1543-1544`). The brief and the phase list use the
OLD numbering. The rows are what matter. Say which numbering a commit message uses.

The new WP-6 carries 5 rows and this design covers 4. The fifth is `RFC7296-3.1-12`, the
DPD I-bit defect, which `plan/spec-fixit-ike-dpd-cleartext.md` has since taken
(`:1576-1579`). Confirm before landing that `3.1-12` is not orphaned between the two.

---

## 6. Risks

| Id | Risk | Signal | Mitigation |
|----|------|--------|------------|
| R-WP5-1 | **An implementer reads `3.2-1` as a receive-side rule and adds a rejection.** `3.2-1` says "MUST be set to zero"; the recipient's rule is the opposite, "MUST be ignored" (`3.2-2`). A rejection would break interoperability with any peer that sets the bit on a defined type | `TestCriticalBitIgnoredForKnownType` reddens | This design adds NO receive-side check. Section 4 states it. The row's tag must say "send side only" |
| R-WP5-2 | **`2.5-12`'s test passes over an empty payload set.** `handleInformationalOwned` builds a response with a nil inner chain, so a naive loop over inner payloads finds nothing and asserts nothing | none; the test is green either way | The anti-vacuity guard in 2.5 is mandatory: assert a non-empty outer chain in one response and a non-empty inner chain in another, and name the counts in the failure message |
| R-WP5-3 | **The harness reaches only one of three generic-header producers.** Producer 3 (`writeAuthHeaderWithMsgID`) writes the SK payload's own header and is easy to miss | the producer-3 mutation leaves the test green | Run all three producer mutations, not a sample. Section 2.6 lists them |
| R-WP5-4 | **Ids are hardcoded from this document and the mark has moved.** Three renumberings have already cost this spec time (`:1363-1387`, `:1399-1414`, `:1416-1438`) | `check_id_allocation` fails naming the id | Recompute the mark at landing with the command in section 5. Land §2.5 rows as a contiguous block |
| R-WP5-5 | **`2.5-5` is classified conformant without an owner ruling.** A classification that lowers what Ze owes is a compliance decision, and `ai/rules/rfc-compliance.md:53` voids prior answers that point away from full compliance | none mechanical | Section 7. Raise it before landing. The pair is written either way |
| R-WP5-6 | **Engine line numbers move under WP-1.** A second agent is editing `internal/component/ike/engine/` now | a tag cites a line that holds different code | Every citation in this design names its function. Re-locate by function name, and re-read before quoting a line in a tag |
| R-WP5-7 | **WP-3 adds an INVALID_MAJOR_VERSION responder and weakens the drop.** RFC 7296 §2.5 pairs "MUST drop" with a SHOULD-send-notify, and `NotifyInvalidMajorVersion` (`wire/payload_notify.go:34`) has no non-test referent today | `2.5-5`'s positive reddens if any non-2 datagram is delivered | The notify must be sent from the drop branch WITHOUT delivering the datagram to an SA. `2.5-5`'s sweep is the guard, and it will catch a regression here |
| R-WP5-8 | **`2.5-5` and `2.5-2` prove the same observable and neither establishes its case.** This is the trap the spec recorded for `1.4.1-4` (`:1816-1841`) | a reviewer cannot tell the two tags apart | `2.5-5`'s sweep covers versions BELOW 2, which `2.5-2` never mentions, and its negative drives the port-4500 producer, which `2.5-2` never reaches. Both facts must be in the tag |
| R-WP5-9 | **The structural `go/ast` assertion in 2.7 is judged gold-plating and dropped, and a later builder sets the bit.** The behavioural tests cover only the messages in the harness | none until a peer complains | Either keep the assertion, or extend `engineBuiltMessages` whenever a builder is added. Say which in review |

No risk in this package alters what Ze ACCEPTS or SENDS, in either direction, because it
adds no production code. That is the direct answer to the brief's question about acceptance
changes: **WP-5 changes nothing about what Ze accepts or emits.** The acceptance risks are
all in the NEIGHBOURING packages that could regress these rows, and R-WP5-1 and R-WP5-7 name
the two concrete ones.

---

## 7. The one item for the owner

**`RFC7296-2.5-5` needs the same ruling OR-D gave `RFC7296-2.5-3`, and I may not extend a
ruling to a row it does not name.**

OR-D reads: "`RFC7296-2.5-3` is discharged by proof, never by annotation. Ze supports the
singleton {2}, and the inbound gate is an equality test on the raw header byte. A tagged
pair must assert that Ze accepts major version 2 and drops every other value. The row stays
gated, so a future second supported version cannot pass unnoticed."
(`plan/spec-rfcgate-1b-rfc7296-pilot.md:1642-1645`.)

`2.5-5` sits on the identical fact, at the identical two producers, and its proof is the
identical sweep. The difference is only which consequence the singleton set discharges:
`2.5-3` needs no intermediate version because there is no interior; `2.5-5` needs no
downgrade recovery because the downgrade state is unreachable.

**What is being asked.** Not permission to skip anything. WP-5 writes the full tagged pair
either way, and the row stays gated. The question is whether the owner reads `2.5-5` as
discharged by that proof, or wants something more.

**What "something more" would cost.** Implementing the consequent literally means
reconnecting at major version 3. No such protocol version exists; RFC 7296 defines 2.0.
Implementing the antecedent means adding support for a second major version, which is a
larger change than this entire spec and has no protocol to target. Neither is a scope
question. Both are unavailable.

Present it as: "OR-D's reasoning applies verbatim to `RFC7296-2.5-5`. Which way do you want
it recorded -- as an extension of OR-D, or as its own ruling?"

---

## 8. Does OR-D's `RFC7296-2.5-3` belong in this package?

**Yes.** Three reasons, and the third is the decisive one.

1. **Same producers.** `2.5-3`'s proof is the equality test at `register.go:645` and
   `register.go:466`. Those are exactly the two lines `2.5-5`'s pair drives, and they are
   the only two version guards in the tree.
2. **Same test.** OR-D specifies "a tagged pair must assert that Ze accepts major version 2
   and drops every other value". That is `TestSupportedMajorVersionSetIsSingleton`,
   character for character. Homing `2.5-3` elsewhere means writing that sweep twice.
3. **Splitting them creates the failure this spec keeps recording.** Two tests over the
   same two lines with the same mutation is duplicated coverage that drifts. Worse, if
   `2.5-3` lands in a different package it must claim a §2.5 ordinal, and section 5 shows
   that a §2.5 row landing alone strands ordinals for everyone else. `2.5-3` is already one
   of the five competitors.

**Recommendation: fold `RFC7296-2.5-3` into WP-5.** One test file, one sweep, three tags
(`2.5-3`, `2.5-5` and the reused `2.5-2` evidence), and one contiguous §2.5 block. If
`2.5-4` ("MUST respond with that version number") can be pulled in with it, the whole §2.5
remainder except WP-3's `2.5-10` lands in one commit, and the ordinal problem disappears.
`2.5-4` is satisfied by the same four builders that hardcode `MajorVersion: 2`, so its pair
is two more assertions over `engineBuiltMessages`, not a new harness.

That is a proposal about scope, so the owner decides. It is strictly MORE work in one
package, never less, so it needs no permission under `ai/rules/no-asking.md`. Raise it with
the `2.5-5` question in section 7 so both are answered together.

---

## 9. Summary row texts to add

Land them in section order in the checklist of `rfc/short/rfc7296.md`: the two §2.5 rows
after `RFC7296-2.5-13`, the §3.2 row after `RFC7296-3.2-3`, the §3.12 row after
`RFC7296-3.12-3`. Ordinals below assume WP-5 lands the §2.5 block first; recompute per
section 5.

    - [ ] [RFC7296-2.5-14] [MUST] If they mistakenly (perhaps through an active attacker
      sending error messages) negotiate to version n, then both will notice that the other
      side can support a higher version number, and they MUST break the connection and
      reconnect using version n+1 (§2.5)

    - [ ] [RFC7296-2.5-15] [MUST NOT] Payloads sent in IKE response messages MUST NOT have
      the critical flag set (§2.5)

    - [ ] [RFC7296-3.2-4] [MUST] The Critical bit MUST be set to zero for payload types
      defined in this document (§3.2)

    - [ ] [RFC7296-3.12-4] [MUST NOT] A Vendor ID payload MUST NOT change the interpretation
      of any information defined in this specification (i.e., the critical bit MUST be set
      to 0) (§3.12)

Each row is one physical line in the file. The wrapping above is for this document only:
`parse_checklist_line` reads one row per line.

`parse_checklist_line` validates that the id's section segment agrees with the `(§X.Y)`
citation, so the citations above are not decoration. `2.5-14` cites §2.5 alone, because the
downgrade sentence appears only there.

After the rows land, run `make ze-rfc-index` and commit `ai/RFC-REQUIREMENTS.md` in the SAME
commit. The ledger records each tagged test's `file:line`, and both verify modes of
`ze-rfc-check` fail on a stale ledger (`ai/rules/testing.md`, RFC-Tagged Tests).

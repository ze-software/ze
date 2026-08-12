<!-- ste: ignore-file preserved verbatim from a design pass; it quotes RFC 7296 at length. -->

# WP-10 design -- certificates, identities, management interface

Rows: `RFC7296-3.6-1`, `-2`, `-3`, `RFC7296-3.5-2`, `-3`, `-4`, `RFC7296-3.3.4-1`,
`RFC7296-4-4`, `RFC7296-2.15-3`.
Source spec: the rfcgate-1b RFC 7296 pilot spec, phase list item 14.

**Read-only design. No tracked file was modified.** Every `file:line` below was read in the
working tree on 2026-07-31. Other agents are editing `internal/component/ike/engine/`, so
engine line numbers move. Every citation names the FUNCTION as well as the line, and the
implementer must re-locate by function name.

**A row-count discrepancy the implementer will meet.** The phase list gives WP-10 nine rows
(`:717`). The 2026-07-30 re-triage table gives "WP-10 | Identity and certificate
configuration | 8" (`:1574`). The two disagree by one, and the re-triage also created a
separate "WP-9 | Crypto suite policy and management facility | 6" (`:1573`), which is the
natural home for `RFC7296-3.3.4-1`. This design covers all nine, and section 9 states the
consequence of `3.3.4-1` moving to WP-9. Say which numbering a commit message uses.

---

## 0. Verdict

| Row | Appendix A class | This design's verdict | Needs production code? |
|-----|------------------|-----------------------|------------------------|
| `RFC7296-3.5-2` | **NOT IMPL** | **conformant.** `encodeIKEID` sends ID_IPV4_ADDR or ID_FQDN under operator control, and the obligation is "at least one of" four | no |
| `RFC7296-3.5-3` | **NOT IMPL** | **conformant.** `remoteIDMatches` accepts all four types. One hardening item, not a violation | no |
| `RFC7296-3.5-4` | **NOT IMPL** | **conformant.** `assertedAddr` and `remoteIDMatches` both handle ID_IPV6_ADDR | no |
| `RFC7296-3.3.4-1` | **NOT IMPL** | **conformant.** The `ike-group / proposal` YANG list IS the management facility, and `3.3.4-2`/`-3` are already proven against it | no |
| `RFC7296-3.6-1` | **NOT IMPL** | **partial.** Send caps at 2 certificates and cannot reach 4. Accept is UNBOUNDED and cannot be configured at all | **yes** |
| `RFC7296-4-4` | **NOT IMPL** | **partial.** RSA-1024/2048 and shared secret both work. The ID-type clause fails for ID_KEY_ID and ID_DER_ASN1_DN | **yes** |
| `RFC7296-2.15-3` | **NOT IMPL** | **absent.** No hex decoding anywhere in the management interface | **yes** |
| `RFC7296-3.6-2` | **NOT IMPL** | **absent.** One of the two Hash-and-URL encodings has a constant with no referent; the other has no constant | **yes** |
| `RFC7296-3.6-3` | **NOT IMPL** | **absent.** Consequent of `3.6-2` | **yes** |

**Four of nine are conformant today and need only tagged tests.** Five need production code.
The package therefore splits cleanly into a proof half and a build half, and section 10
recommends landing them as two commits.

**One row must renumber.** `RFC7296-3.3.4-1` is below its section's committed high-water
mark and `check_id_allocation` refuses it. See section 8.

**The security surface is concentrated in one row pair.** `3.6-2`/`-3` are the only rows
that would make Ze fetch an attacker-named URL. Section 7 shows they can be satisfied
without ever performing a fetch, and states the bound if the fetch is built anyway.

---

## 1. `RFC7296-3.5-2` -- configurable to SEND one of the four types

### The obligation, verbatim

> "Two implementations will interoperate only if each can generate a
> type of ID acceptable to the other.  To assure maximum
> interoperability, implementations MUST be configurable to send at
> least one of ID_IPV4_ADDR, ID_FQDN, ID_RFC822_ADDR, or ID_KEY_ID, and
> MUST be configurable to accept all of these four types."

`rfc/full/rfc7296.txt:5108-5112`. The MUST for this row is on `:5110-5111`.

Appendix A splits that compound sentence at the comma, giving `3.5-2` the send half and
`3.5-3` the accept half. The split is legitimate: two MUSTs, two obligations, one sentence.
Both halves quote their own clause accurately.

The immediately following sentence is a SHOULD and is deliberately not a row:

> "Implementations SHOULD be capable of generating and accepting all of
> these types."

`rfc/full/rfc7296.txt:5113-5114`. Do not let this bleed into `3.5-2`'s tag. The MUST is
"at least one"; the SHOULD is "all". Conflating them turns a satisfied MUST into a
manufactured gap.

### What Ze does today

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| The local identity string is operator-supplied | `parseAuthConfig` | `internal/component/ike/ipsec/config.go` (`auth.LocalID = v` from leaf `local-id`) |
| The YANG leaf that carries it | leaf `local-id` | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` |
| The ID payload is built from it | `buildIDPayload` | `internal/component/ike/engine/auth.go` (`idStr = sa.PeerCfg.Auth.LocalID` at `:574-576`) |
| The type is derived from the string's SHAPE | `encodeIKEID` | `internal/component/ike/engine/auth.go` |
| An IPv4 literal becomes ID_IPV4_ADDR | `encodeIKEID` | `auth.go` |
| An IPv6 literal becomes ID_IPV6_ADDR | `encodeIKEID` | `auth.go` |
| Anything else becomes ID_FQDN | `encodeIKEID` | `auth.go` |
| Nothing ever sends ID_RFC822_ADDR or ID_KEY_ID | `encodeIKEID` | `auth.go`, the function has no branch producing either |

`buildIDPayload` has three non-test callers: `computeSignedOctets` (`auth.go`),
`buildAuthRequest` (`auth.go`), the responder IKE_AUTH (`responder.go`), and the
responder EAP path (`responder_eap.go`). All four route through `encodeIKEID`, so there
is exactly ONE type-selection producer. This is not the second-producer shape.

### Is it conformant?

**Yes.** The obligation is a disjunction: "configurable to send at least one of
ID_IPV4_ADDR, ID_FQDN, ID_RFC822_ADDR, or ID_KEY_ID". Ze is configurable to send
ID_IPV4_ADDR (write an IPv4 literal into `local-id`) and configurable to send ID_FQDN
(write anything that is not an IP literal). Two of the four disjuncts are reachable under
operator control, and one suffices.

The argument rests on a property the code HAS -- `encodeIKEID` has a branch for each of two
named types, driven by a config leaf -- not on the absence of a guard. It does not expire.

**What is NOT claimed.** Ze cannot send ID_RFC822_ADDR, ID_KEY_ID, or ID_DER_ASN1_DN under
ANY configuration: `encodeIKEID` has exactly three return statements (`auth.go`, `:595`,
`:597`) and none produces those types, so an email-shaped `local-id` goes out as ID_FQDN.
That fails the SHOULD at `:5113-5114`, and it is load-bearing for `RFC7296-4-4` (section 6).
It does not fail `3.5-2`.

### A constraint on `local-id` that the tag must name

`ValidatePKIRefs` (`internal/component/ike/ipsec/validate.go`) refuses an X.509 peer whose
`local-id` does not equal the certificate's common name (`validate.go`):

    if peer.Auth.Mode == AuthX509 && peer.Auth.LocalID != "" {
        cn := certCN(cert)
        if cn != "" && peer.Auth.LocalID != cn { ... }

So in X.509 mode the operator's freedom over `local-id` -- and therefore over the ID TYPE Ze
sends -- is bounded by the certificate. An operator wanting ID_IPV4_ADDR must hold a
certificate whose CN is that address literal. That is a real interoperability constraint,
it is Ze's own policy rather than an RFC rule, and `3.5-2` still holds because PSK and EAP
modes are unconstrained and X.509 with a matching CN reaches both types. Name it in the tag
so a reader does not later "discover" it as a contradiction.

### The tagged pair

Add to `internal/component/ike/engine/rfc7296_identity_test.go` (new file; `auth_test.go`
already holds unrelated AUTH-computation tests and `remote_id_test.go` owns the accept
side, so the send side wants its own home).

**Positive -- `TestLocalIDTypeFollowsConfiguredIdentity`.**
Table-driven over `buildIDPayload` through a real SA whose `PeerCfg.Auth.LocalID` is set:
`"10.0.0.1"` yields `IDType == wire.IDTypeIPv4Addr` with `IDData` the four packed octets;
`"2001:db8::1"` yields `IDTypeIPv6Addr` with sixteen octets; `"gw.example.com"` yields
`IDTypeFQDN` with the exact ASCII bytes. Assert `IDData` byte-for-byte, not just the type:
an implementation that picked the right type and sent the wrong octets would interoperate
with nobody, and the length assertion is what catches it.

The tag states the reading: the obligation is a disjunction over four types, Ze reaches two
of them under `local-id`, and one satisfies the MUST. Name the SHOULD at `:5113-5114`
explicitly as NOT claimed, so a later reader does not treat this pass as proof of the
stronger sentence.

**Negative -- `TestLocalIDIsOperatorControlledNotDerived`.**
This is the discriminating half. Assert that the type is a CONSEQUENCE OF CONFIG and not a
constant:

1. With `LocalID` empty, `buildIDPayload` falls back to `sa.PeerName` (`auth.go`).
   Assert the payload then carries the peer name as ID_FQDN. This proves the fallback is
   itself a real path, and it documents that a peer named `10.0.0.1` silently sends
   ID_IPV4_ADDR -- which is a genuine operator trap worth pinning.
2. Two SAs differing ONLY in `LocalID` produce DIFFERENT `IDType` values. Without this, a
   test asserting "IPv4 literal yields ID_IPV4_ADDR" would pass against a function that
   returned `IDTypeIPv4Addr` unconditionally for a fixture that happens to be an address.

### Mutations

| Mutation | Site | Must redden |
|----------|------|-------------|
| `return wire.IDTypeIPv4Addr, v4` becomes `return wire.IDTypeFQDN, []byte(id)` | `encodeIKEID`, `auth.go` | the positive (IPv4 row) and the negative (the two SAs stop differing) |
| `return wire.IDTypeFQDN, []byte(id)` becomes `return wire.IDTypeFQDN, nil` | `encodeIKEID`, `auth.go` | the positive's `IDData` assertion ONLY. This is the mutation that proves the byte assertion earns its place |
| `idStr = sa.PeerCfg.Auth.LocalID` is deleted | `buildIDPayload`, `auth.go` | the negative. `local-id` stops steering the type, and "operator-controlled" is false |
| `encodeIKEID` returns `IDTypeIPv4Addr` unconditionally | `auth.go` | the negative ONLY, if the positive's fixtures were address-shaped. Run it: it is the check that the positive is not vacuous |

---

## 2. `RFC7296-3.5-3` -- configurable to ACCEPT all four types

### The obligation, verbatim

> "MUST be configurable to accept all of these four types."

`rfc/full/rfc7296.txt:5112`, continuing the sentence begun at `:5110`. "These four types"
resolves to ID_IPV4_ADDR, ID_FQDN, ID_RFC822_ADDR, ID_KEY_ID.

Appendix A's row text reads "Implementations MUST be configurable to accept all of these
four types (§3.5)". It restores the subject "Implementations" from `:5110`. That is a
subject restoration, not an overstatement. Keep it, but make the tag name the four types
explicitly, because "these four" is unresolvable out of context.

### What Ze does today

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| The accept-side policy gate | `checkRemoteIdentity` | `internal/component/ike/engine/remote_id.go` |
| Which types can be compared at all | `assertedIdentity` | `remote_id.go` -- ID_IPV4_ADDR, ID_IPV6_ADDR, ID_FQDN, ID_RFC822_ADDR, ID_KEY_ID return `comparable == true` |
| The comparison itself | `remoteIDMatches` | `remote_id.go` |
| ID_IPV4_ADDR / ID_IPV6_ADDR compare as addresses | `remoteIDMatches` | `remote_id.go`, via `assertedAddr` (`remote_id.go`) |
| ID_FQDN / ID_RFC822_ADDR compare as case-folded ASCII | `remoteIDMatches` | `remote_id.go` -- ONE case arm covers both |
| ID_KEY_ID compares as exact octets | `remoteIDMatches` | `remote_id.go` |
| ID_DER_ASN1_DN / ID_DER_ASN1_GN are refused | `assertedIdentity` default arm | `remote_id.go` returns `comparable == false`; `checkRemoteIdentity` then errors at `:247-251` |
| An unset `remote-id` accepts every type | `checkRemoteIdentity` | `remote_id.go` returns nil before any type test |
| The YANG leaf | leaf `remote-id` | `ze-ipsec-conf.yang` |

The package doc comment states the design intent verbatim (`remote_id.go`): "Ze
compares five of the seven types RFC 7296 Section 3.5 assigns ... Those are the four every
implementation MUST accept, plus the address type an IPv6-capable implementation MUST
accept."

**Do not cite that comment as evidence.** `ai/rules/evidence.md` bans treating a
comment as the design record. It happens to be TRUE here, and the evidence is
`remoteIDMatches`' four reachable arms, which I read.

### Is it conformant?

**Yes.** All four MUST-accept types reach a real comparison in `remoteIDMatches`, and the
comparison is driven by the operator's `remote-id` leaf. That is "configurable to accept
all of these four types".

### The hardening item that is NOT a violation

`remoteIDMatches` folds ID_FQDN and ID_RFC822_ADDR into one case arm (`remote_id.go`).
An operator who writes `remote-id user@example.com` therefore accepts a peer asserting
EITHER type, and cannot express "ID_RFC822_ADDR only". The same is true of ID_KEY_ID versus
the text types once the value is byte-equal, gated only by the exact-versus-folded
comparison.

`configuredClass` (`remote_id.go`) does separate the ADDRESS class from the TEXT
class, and `remoteIDMatches` refuses a cross-class match at `:204-207`. So the widening is
confined to within the text class. That is three types collapsing to one acceptance
decision, not seven.

**This does not violate `3.5-3`, which requires acceptance and says nothing about
restriction.** It is a fail-open widening of an identity check, so
`ai/rules/evidence.md` has an interest in it, and section 9 files it as the
package's one deliberate non-goal with a named home.

### The tagged pair

Add to `internal/component/ike/engine/remote_id_test.go`, which already owns this surface.

**Positive -- `TestRemoteIDAcceptsEveryMandatoryType`.**
Four sub-tests, one per MUST-accept type. For each: configure `remote-id` to a value of that
type's shape, build a `wire.PayloadID` of that type carrying the matching octets, and assert
`checkRemoteIdentity` returns nil. Use `ID_KEY_ID` with a non-UTF8 opaque value (say
`[]byte{0x00, 0xff, 0x41}`) so the exact-octet arm at `:221-222` is genuinely exercised and
not accidentally satisfied by the text arm.

**Anti-vacuity guard, mandatory.** `checkRemoteIdentity` returns nil when `remote-id` is
empty (`remote_id.go`). A sub-test that forgot to set `remote-id` would pass for
every type including ID_DER_ASN1_DN, and the whole positive would be vacuous. The test MUST
assert, once per sub-test, that the same payload with a DELIBERATELY WRONG `remote-id` is
refused. Without that, this test proves nothing at all.

**Negative -- `TestRemoteIDRefusesTypesItCannotCompare`.**
Assert that ID_DER_ASN1_DN (9) and ID_DER_ASN1_GN (10) are REFUSED with an error naming the
five comparable types, and that the refusal is the `comparable == false` branch
(`remote_id.go`), not a value mismatch. Assert the error text contains
`"cannot compare"`. This is the discriminating half: it proves the positive's four passes
are a property of those four types rather than of a check that accepts everything.

### Mutations

| Mutation | Site | Must redden |
|----------|------|-------------|
| The `IDTypeRFC822Addr` case label is deleted from `:219` | `remoteIDMatches` | the positive's RFC822 sub-test |
| The `IDTypeKeyID` arm returns false | `remoteIDMatches`, `remote_id.go` | the positive's KEY_ID sub-test |
| `assertedIdentity` returns `comparable == true` in its default arm | `remote_id.go` | the negative |
| `checkRemoteIdentity` returns nil unconditionally | `remote_id.go` | the anti-vacuity guard in the positive AND the negative. If this mutation leaves the positive green, the guard was not written |

---

## 3. `RFC7296-3.5-4` -- IPv6-capable implementations and ID_IPV6_ADDR

### The obligation, verbatim

> "IPv6-capable implementations MUST additionally be
> configurable to accept ID_IPV6_ADDR."

`rfc/full/rfc7296.txt:5114-5115`.

The following sentence is a MAY and is not a row:

> "IPv6-only implementations MAY
> be configurable to send only ID_IPV6_ADDR instead of ID_IPV4_ADDR for
> IP addresses."

`rfc/full/rfc7296.txt:5115-5117`.

### The antecedent: is Ze IPv6-capable?

The obligation is conditional. Establish the antecedent before the consequent, and do it
from code rather than from assumption.

| Evidence | `file:line` |
|----------|-------------|
| `encodeIKEID` emits ID_IPV6_ADDR for an IPv6 literal | `internal/component/ike/engine/auth.go` |
| `assertedAddr` accepts a 16-octet ID_IPV6_ADDR | `internal/component/ike/engine/remote_id.go` |
| `assertedIdentity` reports ID_IPV6_ADDR comparable | `remote_id.go` |
| `remoteIDMatches` compares it as `netip.Addr` | `remote_id.go` |
| `idTypeName` names it | `remote_id.go` |

Ze is IPv6-capable in the sense this section means: it both generates and accepts an IPv6
identity. The antecedent holds, so the consequent is owed.

### Is it conformant?

**Yes.** `remoteIDMatches` compares ID_IPV6_ADDR under `remote-id`, exactly as it compares
ID_IPV4_ADDR. `assertedAddr` enforces the 16-octet length RFC 7296 Section 3.5 assigns
(`remote_id.go`), so a truncated or over-long payload is refused rather than
silently zero-padded.

One nuance worth a line in the tag: `assertedAddr` returns `addr.Unmap()`
(`remote_id.go`), and `remoteIDMatches` unmaps the configured value too. So a
peer sending `::ffff:10.0.0.1` as a 16-octet ID_IPV6_ADDR matches a `remote-id` of
`10.0.0.1`. That is deliberate and documented at `remote_id.go`. It is an acceptance
WIDENING across the v4/v6 boundary within the address class. It does not violate `3.5-4`,
and it belongs in the tag so a reviewer is not surprised by it.

### The tagged pair

**Positive -- `TestRemoteIDAcceptsIPv6Identity`.**
`remote-id 2001:db8::1` plus an ID_IPV6_ADDR payload carrying those sixteen octets returns
nil from `checkRemoteIdentity`. Include the send half: `encodeIKEID("2001:db8::1")` yields
`IDTypeIPv6Addr` and sixteen octets, so the row's "configurable" is proven at both ends of
one configuration.

**Negative -- `TestIPv6IdentityLengthIsEnforced`.**
An ID_IPV6_ADDR payload carrying 4 octets, or 15, or 17, is REFUSED. Assert the refusal is
the not-comparable branch. This proves the positive's pass depends on the payload actually
being a well-formed IPv6 address, and not on the type octet alone. Add one row asserting
that an ID_IPV4_ADDR payload of 16 octets is also refused, which pins the per-type length
table rather than a single length constant.

### Mutations

| Mutation | Site | Must redden |
|----------|------|-------------|
| `len(p.IDData) != 16` becomes `len(p.IDData) < 4` | `assertedAddr`, `remote_id.go` | the negative |
| The `IDTypeIPv6Addr` case is deleted from `assertedAddr` | `remote_id.go` | the positive |
| `encodeIKEID`'s IPv6 branch returns `IDTypeFQDN` | `auth.go` | the positive's send half |

---

## 4. `RFC7296-3.3.4-1` -- the management facility for acceptable IKE suites

### The obligation, verbatim

> "All implementations of IKEv2 MUST include a management facility that
> enables a user or system administrator to specify the suites that are
> acceptable for use with IKE.  Upon receipt of a payload with a set of
> Transform IDs, the implementation MUST compare the transmitted
> Transform IDs against those locally configured via the management
> controls, to verify that the proposed suite is acceptable based on
> local policy.  The implementation MUST reject SA proposals that are
> not authorized by these IKE suite controls."

`rfc/full/rfc7296.txt:4771-4778`. `3.3.4-1` is the first sentence; `3.3.4-2`
is the second; `3.3.4-3` is the third.

The trailing sentence bounds all three and should be in the tag:

> "Note that cryptographic
> suites that MUST be implemented need not be configured as acceptable
> to local policy."

`rfc/full/rfc7296.txt:4778-4780`.

### The decisive fact: its two dependents are already proven

`RFC7296-3.3.4-2` and `RFC7296-3.3.4-3` are committed at HEAD and carry tagged pairs:

| Row | Positive | Negative |
|-----|----------|----------|
| `3.3.4-2` | `internal/component/ike/crypto/rfc7296_proposal_test.go` | `:125` |
| `3.3.4-3` | `internal/component/ike/crypto/rfc7296_proposal_test.go` | `:161` |

Both are `unit/verify` tier in the ledger (`ai/RFC-REQUIREMENTS.md`).
`TestPropTransformIDsComparedAgainstLocalPolicy` (`rfc7296_proposal_test.go`) drives
`NegotiateIKE(offer, policy)` and asserts that WIDENING the policy changes the outcome
(`:143-154`). That is proof the policy is data rather than a constant.

### What Ze does today

| Property | Producing site | `file:line` |
|----------|----------------|-------------|
| The operator-facing suite list | YANG `list proposal` under `list ike-group` | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` |
| Bounded at 255 proposals, citing RFC 7296 Section 3.3.1 | same | `ze-ipsec-conf.yang` |
| Encryption transform, mandatory | leaf `encryption` | `ze-ipsec-conf.yang` |
| PRF and integrity transform, mandatory | leaf `hash` | `ze-ipsec-conf.yang` |
| Diffie-Hellman group, mandatory, range 1..31 | leaf `dh-group` | `ze-ipsec-conf.yang` |
| Priority ordering | leaf `number`, list key | `ze-ipsec-conf.yang, 186-191` |
| The comparison against that policy | `NegotiateIKE` | `internal/component/ike/crypto/` (proven by `rfc7296_proposal_test.go`) |

### Is it conformant?

**Yes.** The `ike-group / proposal` list IS a management facility by which a system
administrator specifies the suites acceptable for use with IKE. It is reachable from the
config tree, it is per-transform-type, it is priority-ordered, and `3.3.4-2`/`-3` already
prove the negotiator consults it and rejects on it.

Ze's facility is a proposal LIST rather than a free-form transform matrix, which is the
same shape strongSwan and libreswan expose. The RFC names no required shape.

**This row is conformant by a facility the code HAS, and its two dependents' green tests are
the strongest available evidence.** A management facility that did not exist could not have
a widening test that changes an outcome.

### A defect INSIDE the facility, found while establishing the verdict

The facility accepts a suite it cannot honour, and it accepts it silently.

| Fact | `file:line` |
|------|-------------|
| YANG accepts any DH group 1..31 | `ze-ipsec-conf.yang` (`range "1..31"`) |
| `ValidDHGroup` accepts the same range | `internal/component/ike/ipsec/types.go` |
| Only three groups have a transform | `dhGroupRegistry`, `internal/component/ike/crypto/transform.go` -- 14, 19, 20 |
| `encryption` IS gated at parse | `EncryptionImplemented` (`crypto/algorithm_support.go`) called at `config.go` and `:383` |
| `hash` IS gated at parse | `HashImplemented` (`crypto/algorithm_support.go`) called at `config.go` and `:401` |
| **`dh-group` is NOT gated at parse** | `parseIKEProposal` reads it at `config.go` with no registry check |
| A miss returns the ZERO transform, not an error | `lookupEncryption` / `lookupPRF` / `lookupIntegrity`, `initiator.go`, `:346`, `:354` |

`dh-group 5` therefore commits, and the proposal Ze offers or accepts carries a zero DH
transform. `ai/rules/protocol.md` is explicit: a backend that cannot deliver exactly
what the config asks for must fail at verify with a clear error. The two sibling leaves
already do this; `dh-group` was missed.

**This is in scope.** `ai/rules/completion.md` makes a defect I am the entry point for mine
to fix, and this one sits inside the very facility `3.3.4-1` asserts. It is also small: add
a `DHGroupImplemented` beside the two existing predicates and call it from
`parseIKEProposal` at `config.go`, mirroring `config.go` exactly. Fix it in this
package and let the negative test below prove it.

Note this does NOT make `3.3.4-1` non-conformant. The MUST is that a facility exist. The
defect is that the facility under-validates, which is a Ze rule rather than an RFC one.

### The tagged pair

Put it in `internal/component/ike/ipsec/rfc7296_test.go`, beside the `2.15` pair. The row is
about the MANAGEMENT INTERFACE, and that file is already the management-interface home
(`:1-6` doc comment). The crypto package owns the comparison; the ipsec package owns the
facility.

**Positive -- `TestIKESuitePolicyIsOperatorSpecified`.**
Parse a config tree with an `ike-group` carrying two proposals that differ in every
transform type. Assert `ParseIPsecConfig` yields both, in priority order, with each
transform value preserved exactly as written. Then parse a SECOND tree naming a different
suite and assert the parsed policy differs. The two-tree differential is what makes this a
facility test rather than a parser test.

**Negative -- `TestIKESuitePolicyRejectsAnUnhonourableSuite`.**
The three transform leaves are `mandatory true` (`ze-ipsec-conf.yang, 201, 214`). Assert:

1. A proposal omitting `dh-group` is REFUSED.
2. `dh-group 0` and `dh-group 32` are refused by `range "1..31"`.
3. **`dh-group 5` is refused** -- the defect above. 5 is inside the YANG range, passes
   `ValidDHGroup`, and has no entry in `dhGroupRegistry` (`crypto/transform.go`).
   Today it commits. After the fix it must be refused at parse with an error naming the
   group and the implemented set, exactly as `encryption chacha20poly1305` is refused today.
4. `dh-group 14`, `19` and `20` are ACCEPTED, so the new gate is a filter and not a blanket
   refusal.

Rows 3 and 4 together are what make this a facility test. A management facility that accepts
a suite the negotiator cannot honour is not "specifying the suites that are acceptable" --
it is recording a wish.

### Mutations

| Mutation | Site | Must redden |
|----------|------|-------------|
| The `proposal` list parse drops `dh-group` | `parseIKEProposal`, `config.go` | the positive's exact-preservation assertion |
| `range "1..31"` is widened to `0..255` | `ze-ipsec-conf.yang` | the negative's row 2 |
| `mandatory true` is removed from `dh-group` | `ze-ipsec-conf.yang` | the negative's row 1 |
| The new `DHGroupImplemented` call is deleted | `parseIKEProposal`, `config.go` | the negative's row 3 ONLY. This is the mutation that proves the defect fix is live |
| `DHGroupImplemented` returns true always | `crypto/algorithm_support.go` | the negative's row 3 |
| `DHGroupImplemented` returns false always | same | the negative's row 4, which is why row 4 exists |
| The parser returns a hardcoded proposal list, ignoring the tree | `parseIKEGroup`, `config.go` | the positive's two-tree differential ONLY. Run this one: it is the mutation the single-tree half cannot see |

---

## 5. `RFC7296-3.6-1` -- up to four X.509 certificates, send and accept

### The obligation, verbatim

> "Implementations MUST be capable of being configured to send and
> accept up to four X.509 certificates in support of authentication,
> and also MUST be capable of being configured to send and accept the
> two Hash and URL formats (with HTTP URLs).  If multiple certificates
> are sent, the first certificate MUST contain the public key
> associated with the private key used to sign the AUTH payload.  The
> other certificates may be sent in any order."

`rfc/full/rfc7296.txt:5271-5277`. `3.6-1` is `:5271-5273`; `3.6-2` is `:5273-5274`; the
first-certificate rule at `:5274-5276` is `RFC7296-1.2-2`, already proven (section 9).

### What Ze does today: the send side caps at TWO, in the PKI layer

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| The CERT payload chain Ze sends | `buildCertPayloads` | `internal/component/ike/engine/auth.go` |
| Capacity hint says two | `buildCertPayloads` | `auth.go` (`make([]wire.PayloadEntry, 0, 2)`) |
| The leaf, always first | `buildCertPayloads` | `auth.go` |
| At most ONE intermediate | `buildCertPayloads` | `auth.go`, guarded by `len(entry.RawInter) > 0` |
| The structural cause: the PKI entry holds ONE intermediate | `CertificateEntry` | `internal/component/pki/types.go` (`Intermediate *x509.Certificate`, `RawInter []byte` -- both singular) |
| The YANG leaf is singular | `parseCertificate` | `internal/component/pki/config.go` (`tree.Get("intermediate")`, one value) |

**The send cap is not in the IKE layer.** `buildCertPayloads` faithfully sends everything
the PKI entry holds. The entry can hold two certificates and no more, so the maximum Ze can
be CONFIGURED to send is two. Raising it to four is a change to `internal/component/pki/`,
and only then to `buildCertPayloads`.

### What Ze does today: the accept side is UNBOUNDED

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| CERT payloads are collected into a slice | initiator path | `internal/component/ike/engine/fsm.go` |
| Same, responder path | responder path | `internal/component/ike/engine/responder.go` |
| Both filter on encoding and non-empty data | both | `fsm.go` (`p.CertEncoding == wire.CertEncodingX509Sig && len(p.CertData) > 0`) |
| The slice is stored with no count limit | `storeRemoteCerts` | `internal/component/ike/engine/auth.go` |
| First becomes the peer certificate | `storeRemoteCerts` | `auth.go` |
| Every later one becomes an intermediate | `storeRemoteCerts` | `auth.go`, an unbounded `append` |
| All of them are parsed and pooled | `getRemoteCert` | `auth.go` |

There is no cap at any of the four sites. A peer may send as many CERT payloads as fit in
an IKE message, and Ze will parse every one into an `x509.CertPool`
(`auth.go`).

### Is it conformant?

**No, partial, and in two opposite directions.**

- **Send:** the maximum is 2. The RFC requires "capable of being configured to send ... up
  to four". Ze cannot be configured to send three or four.
- **Accept:** the maximum is unbounded. "Capable of being configured to ... accept up to
  four" describes a CONFIGURABLE bound. Ze has no bound at all, so it is not configurable to
  accept four -- it accepts four by accident of having no limit, which is a different
  property and a worse one.

The spec anticipated exactly this and named it a security risk: "Accepting up to four
certificates means accepting an attacker-supplied chain: the first certificate MUST carry
the AUTH key (`RFC7296-1.2-2`) and chain validation must not be weakened to accommodate the
new count" (the rfcgate-1b RFC 7296 pilot spec).

### Production design

**Layer 1 -- `internal/component/pki/`, the send-side capacity.**

| Change | File, function |
|--------|----------------|
| `CertificateEntry.Intermediate` / `.RawInter` become slices | `internal/component/pki/types.go` |
| The YANG `leaf intermediate` becomes a `leaf-list` (or a keyed `list`) | `internal/component/pki/yang/ze-pki-conf.yang` |
| Parse every value instead of one | `internal/component/pki/config.go`, `parseCertificate` |
| Every existing reader of `RawInter` follows the type change | `internal/component/pki/show.go`, `:197-198`, `internal/component/pki/store.go` |

This is the sibling call-site audit `ai/rules/architecture.md` requires: `RawInter`
has four non-test readers and all four must change together. The `show` and `store` readers
build PEM bundles and are already loop-friendly.

**A YANG ordering decision the implementer must make deliberately.** A `leaf-list` in the
config tree has an order, and RFC 7296 says the certificates after the first "may be sent in
any order" (`:5276-5277`), so order does not matter on the wire. It DOES matter for a config
diff. Prefer a `leaf-list` with `ordered-by user` so a reordering is a visible change rather
than a silent one.

**Layer 2 -- `internal/component/ike/engine/auth.go`, the send-side assembly.**

`buildCertPayloads` (`auth.go`) loops over the intermediate slice instead of testing
one value. The leaf stays first, unconditionally, because `RFC7296-1.2-2` requires it and
`TestAutFirstCertificateCarriesTheAuthKey` proves it. Change `:612`'s capacity hint to
`1 + len(entry.RawInter)`.

**Layer 3 -- the configurable bound, both directions.**

A new YANG leaf under the peer's `authentication` container (`ze-ipsec-conf.yang`),
named per `ai/rules/config.md` (kebab-case, noun, no unit in the name, no `max-`
prefix that duplicates the type):

| Leaf | Type | Default | Meaning |
|------|------|---------|---------|
| `certificate-count` | `uint8 { range "1..4" }` | `4` | The maximum number of X.509 certificates Ze sends, and the maximum it accepts, for this peer |

The RFC's number is four, and the range encodes it. `ai/rules/config.md` requires
native validation, and `ai/patterns/config-option.md` is the structural template.

**The wiring trap every new `AuthConfig` field in this package must clear.** `peersEqual`
(`internal/component/ike/ipsec/types.go`) decides whether a reloaded peer config counts
as CHANGED, and it compares six auth fields explicitly (`types.go`:
`Mode`, `PSK`, `LocalID`, `RemoteID`, `CACertificate`, `Certificate`). A new field that is
not added there is INERT ON RELOAD: an operator edits `certificate-count`, commits, and
nothing renegotiates. That is exactly the "inert config surface" `ai/rules/completion.md`
names. **Every leaf this package adds -- `certificate-count`, `pre-shared-secret-encoding`,
`remote-id-type`, `hash-and-url`, `certificate-url` -- must land in `peersEqual` in the same
commit, with a reload test.** Note also that `AuthConfig` (`types.go`) carries NO
struct tags at all, so a field holding anything secret needs `json:"-"` explicitly, as
`EAPUser.Password` does at `types.go`.

**Why a default of 4 and not 2.** The RFC's figure is the interoperability floor. A default
of 4 means an operator who never touches the leaf gets exactly the conformant behaviour, and
`ai/rules/config.md`'s "defaults are requirements" applies. A lower default would
make the conformant case opt-in.

**Layer 4 -- enforcing the bound on ACCEPT, and this is the load-bearing half.**

`storeRemoteCerts` (`auth.go`) gains the cap. The design decision is what to do on
overflow, and `ai/rules/evidence.md` decides it: **refuse the message, do not
truncate**.

Truncating is the tempting choice and it is wrong. A peer that sends five certificates
where the first four do not chain, but the fifth does, would authenticate under a
truncating implementation only by luck of ordering; and under a silently-truncating one the
operator never learns the bound was hit. Worse, truncation makes the bound invisible to the
test that is supposed to prove it. Return an error naming the count and the configured
limit, per `ai/rules/cli.md`'s what/why/next.

`storeRemoteCerts` currently returns nothing (`auth.go`). It gains an `error` return,
and both call sites (`fsm.go`, `responder.go`) handle it by moving the SA to
`StateDead` with a warning, which is the shape both sites already use for
`recordPeerWindowSize` immediately below (`fsm.go`, `responder.go`). That
is a two-line change at each site, following an existing local pattern.

**Why the engine layer owns the accept cap.** The cap is per-peer policy, and the peer
config is on the SA. The wire layer cannot see policy, and the PKI layer never sees the
peer's payloads. `storeRemoteCerts` is the single funnel through which every received CERT
payload passes -- both collection loops call it and nothing else writes
`sa.RemoteCertChainRaw`. It is the only place a bound can be enforced once.

### The tagged pair

**Positive -- `TestCertificateCountReachesFourInBothDirections`.**
Two halves, and both are required because the row's obligation is "send AND accept".

1. *Send.* Configure a PKI entry with a leaf and three intermediates. Assert
   `buildCertPayloads` returns exactly four `wire.PayloadEntry` values, that the first is
   the leaf whose key `computeX509Auth` signs with, and that all four carry
   `CertEncodingX509Sig`.
2. *Accept.* Drive a real IKE_AUTH carrying four CERT payloads through the responder path
   and assert `sa.RemoteCertRaw` is the first and `sa.RemoteCertChainRaw` has three
   entries, in wire order.

**Negative -- `TestCertificateCountIsBoundedAndConfigurable`.**
The discriminating half, and the security half.

1. Five CERT payloads with `certificate-count 4` is REFUSED, and the SA does not establish.
   Assert the error names both the received count and the limit.
2. The SAME five payloads with the limit raised are... still refused, because the range is
   `1..4` and 5 is unconfigurable. Assert instead that four payloads with
   `certificate-count 2` is refused, and that the identical four with `certificate-count 4`
   is accepted. That pair proves the bound is READ FROM CONFIG rather than a constant, which
   is precisely what "capable of being CONFIGURED" requires.
3. Assert refusal, not truncation: after the refusal, `sa.RemoteCertChainRaw` must be empty
   and the SA must not be `StateEstablished`. A truncating implementation passes step 1's
   count assertion and fails this one.

### Mutations

| Mutation | Site | Must redden |
|----------|------|-------------|
| The intermediate loop sends only `entry.RawInter[0]` | `buildCertPayloads`, `auth.go` | the positive's send half |
| The leaf is appended after the intermediates | `buildCertPayloads`, `auth.go` | `TestAutFirstCertificateCarriesTheAuthKey` (existing, `rfc7296_auth_test.go`) AND the positive's first-certificate assertion |
| The cap in `storeRemoteCerts` truncates instead of erroring | `auth.go` | the negative's step 3 ONLY. Run it: it is the only mutation that separates a bound from a silent trim |
| The cap reads a constant 4 instead of `sa.PeerCfg` | `storeRemoteCerts` | the negative's step 2 |
| The `storeRemoteCerts` error is dropped at one call site | `fsm.go` or `responder.go` | the negative, but ONLY on the path that site serves. **Mutate BOTH sites separately.** This is the second-producer shape the spec has recorded three times, and there are exactly two producers here |

---

## 6. `RFC7296-4-4` -- the conformance configuration set

### The obligation, verbatim

> "For an implementation to be called conforming to this specification,
> it MUST be possible to configure it to accept the following:
>
> o  Public Key Infrastructure using X.509 (PKIX) Certificates
>    containing and signed by RSA keys of size 1024 or 2048 bits, where
>    the ID passed is any of ID_KEY_ID, ID_FQDN, ID_RFC822_ADDR, or
>    ID_DER_ASN1_DN.
>
> o  Shared key authentication where the ID passed is any of ID_KEY_ID,
>    ID_FQDN, or ID_RFC822_ADDR."

`rfc/full/rfc7296.txt:6874-6883`.

Appendix A's row text is "it MUST be possible to configure it to accept PKIX certificates
containing and signed by RSA keys of size 1024 or 2048 bits, and shared secret
authentication (§4)". **That elision is not safe.** It drops the ID-type clause from BOTH
bullets, and the ID-type clause is where Ze fails. A reader working only from Appendix A
would classify this row conformant. Restore the clause in the summary row text
(section 10).

There is a third bullet the walk did not capture:

> "o  Authentication where the responder is authenticated using PKIX"

`rfc/full/rfc7296.txt:6895`. It continues past the page break. Read `:6895-6910` before
finalising the row text, and raise it with the owner if it carries a separable obligation --
`ai/rules/rfc-compliance.md` Extraction Completeness makes an unextracted obligation still
an obligation, and an enumeration hole is exactly the signal it names.

### What Ze does today

**The RSA half is satisfied.**

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| The private key is parsed from config | `parsePrivateKey` | `internal/component/pki/config.go` |
| Signing selects an algorithm from the key type | `selectSignatureAlgorithm` | `internal/component/ike/engine/auth.go` |
| Verification uses the certificate's public key | `verifyX509Auth` | `auth.go`, `verifySignature` at `:409` |
| Legacy RSA AUTH method 1 is also accepted | `verifyLegacyRSAAuth` | `auth.go` |

Ze imposes no key-size floor or ceiling that would exclude 1024 or 2048. Confirm during
implementation by minting both sizes in the test fixture rather than by reading code -- key
size is enforced, if at all, inside `crypto/x509` and `crypto/rsa`, and the only honest
proof is a round trip. **Do not claim this half conformant from a code read.**

**The shared-key half is satisfied**, via `parseAuthConfig` (`internal/component/ike/ipsec/config.go`)
and `computePSKAuth` (`internal/component/ike/engine/auth.go`), both already proven
by the `2.15-1`/`-2` pairs.

**The ID-type clause FAILS, in two places.**

| Required ID type | PKIX bullet | Shared-key bullet | Ze's behaviour |
|------------------|-------------|-------------------|----------------|
| ID_FQDN | required | required | works. `certificateCarriesIdentity` `remote_id.go`; PSK via `checkRemoteIdentity` |
| ID_RFC822_ADDR | required | required | works. `certificateCarriesIdentity` `remote_id.go` |
| ID_KEY_ID | required | required | **PKIX FAILS.** `certificateCarriesIdentity` has no ID_KEY_ID arm and falls to `return false` at `remote_id.go`. Shared-key works (`remoteIDMatches` `:221-222`) |
| ID_DER_ASN1_DN | required | not required | **FAILS.** `assertedIdentity` returns `comparable == false` (`remote_id.go`), so `checkRemoteIdentity` refuses at `:247-251` before any certificate is read |

The ID_KEY_ID refusal is DELIBERATE and documented (`remote_id.go`): "ID_KEY_ID
never binds. A certificate holds no field that corresponds to an opaque vendor identity, so
the check denies rather than guesses". That reasoning is sound as a fail-closed choice and
it still leaves the MUST unmet: RFC 7296 Section 4 requires it be POSSIBLE TO CONFIGURE Ze
to accept a PKIX certificate where the ID passed is ID_KEY_ID.

### The escape hatch that is not one

With `remote-id` unset, `getRemoteCert` warns and returns the certificate
(`auth.go`), so a PKIX peer asserting ID_KEY_ID or ID_DER_ASN1_DN IS accepted.

**Do not count that as satisfying `4-4`.** It is acceptance by the absence of a check, and
the log line at `:533-537` says so in as many words: "remote-id is not set, so every
certificate this authority issued authenticates as this peer". `ai/rules/evidence.md`
treats that as a guard that cannot deny and says so, which is the correct behaviour for an
unset expectation and is not a configured acceptance of an identity type. A tag arguing
otherwise would be the expiring-negative shape the spec has recorded twice.

### Production design

**This row is the one that needs an owner decision before code, and section 11 states it.**
The narrow reading (accept the two types by binding them) and the wide reading (also let an
operator RESTRICT the accepted type, closing the section-2 widening) differ in cost by
roughly a factor of three, and `ai/rules/rfc-compliance.md` forbids me choosing the narrower
answer alone.

The narrow design, which is what the MUST actually requires:

| Change | File, function |
|--------|----------------|
| **The config-time refusal of a DN must be lifted** | `ValidateIdentities`, `internal/component/ike/ipsec/validate.go` |
| ID_DER_ASN1_DN becomes comparable: parse the DER distinguished name and render it in RFC 4514 string form | `assertedIdentity`, `internal/component/ike/engine/remote_id.go` |
| ID_DER_ASN1_DN binds against the certificate's own `RawSubject` | `certificateCarriesIdentity`, `remote_id.go` -- a new arm comparing the asserted DER to `cert.RawSubject` |
| ID_KEY_ID binds when the operator opts in | `certificateCarriesIdentity`, new arm |
| The YANG `remote-id` description stops saying a DN is refused | `ze-ipsec-conf.yang` |

**The third site is the one that is easy to miss, and it fails FIRST.** `ValidateIdentities`
(`validate.go`) walks `{"local-id", auth.LocalID}` and `{"remote-id", auth.RemoteID}`
(`validate.go`) and REFUSES a DN-shaped value at commit (`validate.go`):

> "is a distinguished name, and ze cannot compare ID_DER_ASN1_DN. Set it to an address, an
> FQDN, a mail address, or a key id"

It is called from `validateIPsecSections` (`internal/component/ike/engine/config.go`),
and the YANG description repeats the rule in prose (`ze-ipsec-conf.yang`: "a
distinguished name is refused at commit"). So a `remote-id` naming a DN never reaches
`assertedIdentity` at all -- the config is rejected before the daemon runs. An implementer
who changes only the two engine functions will watch the positive fail at config parse and
be unable to see why.

`ValidateIdentities` was correct when it was written: it is `ai/rules/protocol.md`
applied honestly, refusing config the engine could not honour. Lifting it is only legitimate
BECAUSE the engine gains the capability in the same commit. **Do not lift it first.** Its
tests (`internal/component/ike/ipsec/validate_identity_test.go`, `:60`, and
`internal/component/ike/engine/config_verify_test.go`
`TestValidateIPsecSectionsRejectsADistinguishedNameRemoteID`) will redden, and each must be
rewritten to assert the new contract rather than deleted -- `ai/rules/testing.md`
makes a rewrite-as-replacement a reviewable event, and `rfc-tagged-test` does not cover
these because they carry no tag.

**ID_DER_ASN1_DN is the tractable one and should be done by DER comparison, not by string
comparison.** `assertedIdentity`'s doc already names the obstacle
(`remote_id.go`): "comparing one against configured text needs a canonical form that
no rule in RFC 7296 states. Ze does not guess at one." That objection is correct for
comparing against operator TEXT, and it dissolves for comparing against the CERTIFICATE:
`cert.RawSubject` is DER and the asserted identity is DER, so `bytes.Equal` is exact and
canonical with nothing guessed. The operator-text comparison in `remoteIDMatches` still
needs a string form, and RFC 4514 via `crypto/x509/pkix.RDNSequence.String()` is the
conventional one -- but note it is a Go-specific rendering, so `remote-id` matching on a DN
must be documented as RFC 4514 form.

**ID_KEY_ID cannot bind to any certificate field, and that is the point.** The RFC requires
that Ze be configurable to ACCEPT such a peer, not that Ze invent an attestation. The
fail-closed-compatible design is an explicit opt-in leaf that says the operator accepts
authority-level trust for this peer:

| Leaf | Type | Default | Meaning |
|------|------|---------|---------|
| `remote-id-type` | `enumeration { ipv4-address; ipv6-address; fqdn; rfc822-address; key-id; der-asn1-dn }` | absent | When set, Ze accepts ONLY this ID type from the peer. When absent, current behaviour |

With `remote-id-type key-id` plus `remote-id <value>`, `checkRemoteIdentity` proves the peer
asserted that exact opaque value, and `certificateCarriesIdentity` returns true for
ID_KEY_ID because the operator has stated that the chain to `ca-certificate` plus the exact
key-id IS the intended binding. That is an operator decision recorded in config, which is
what "possible to configure it to accept" means, and it is not a silent widening.

**This one leaf also closes section 2's hardening item**, because
`remote-id-type rfc822-address` makes ID_FQDN with the same text refusable. One leaf, two
problems, and it is the wide reading's main cost saving.

### The tagged pair

**Positive -- `TestConformanceConfigurationSetIsAcceptable`.**
Four sub-tests, one per required combination, each a full authentication:

1. PKIX with an RSA-1024 key, ID_FQDN.
2. PKIX with an RSA-2048 key, ID_RFC822_ADDR.
3. PKIX with an RSA-2048 key, ID_DER_ASN1_DN.
4. PKIX with an RSA-2048 key, ID_KEY_ID, with `remote-id-type key-id` configured.
5. Shared key with ID_KEY_ID, and again with ID_FQDN and ID_RFC822_ADDR.

Mint both RSA sizes in the fixture. That is the only honest proof of the key-size half, and
it is cheap: `rsa.GenerateKey(rand.Reader, 1024)` is fast enough for a unit test.

**Negative -- `TestConformanceSetDoesNotAcceptWhatItMustNot`.**
Two halves:

1. Each of the four PKIX sub-tests fails when the certificate does NOT carry the asserted
   identity. Otherwise the positive would pass against a check that accepts every
   certificate the authority issued, which is exactly the state `remote-id` exists to fix.
2. `remote-id` UNSET is not the same as configured acceptance. Assert that an unset
   `remote-id` logs the "every certificate this authority issued authenticates as this
   peer" warning (`auth.go`). Pin the warning, because if it ever disappears the
   fail-open path becomes silent and this row's evidence becomes indistinguishable from it.

### Mutations

| Mutation | Site | Must redden |
|----------|------|-------------|
| The ID_DER_ASN1_DN arm compares rendered strings instead of DER | `certificateCarriesIdentity` | the negative's half 1, via a DN whose rendering collides |
| The ID_KEY_ID arm returns true regardless of `remote-id-type` | `certificateCarriesIdentity` | the negative's half 1 |
| `assertedIdentity` reports ID_DER_ASN1_DN not comparable again | `remote_id.go` | the positive's sub-test 3 |
| The RSA-1024 fixture is swapped for 2048 | the test's own fixture | nothing -- which is the point. If sub-test 1 stays green after this, the key-size half is untested and the fixture must be parameterised |

---

## 7. `RFC7296-3.6-2` and `-3` -- Hash and URL, and the SSRF surface

**This section is the security core of the package. Read it before writing any code.**

### The obligations, verbatim

**`3.6-2`:**

> "and also MUST be capable of being configured to send and accept the
> two Hash and URL formats (with HTTP URLs)."

`rfc/full/rfc7296.txt:5273-5274`.

**`3.6-3`:**

> "Implementations MUST support the "http:" scheme for hash-and-URL
> lookup.  The behavior of other URL schemes [URLS] is not currently
> specified, and such schemes SHOULD NOT be used in the absence of a
> document specifying them."

`rfc/full/rfc7296.txt:5279-5282`. Appendix A cites this row as "(§3.6, §1.7)", and §1.7
confirms it is a change introduced by RFC 7296 over RFC 5996:

> "In Section 3.6, "Implementations MUST support the "http:" scheme for
> hash-and-URL lookup.  The behavior of other URL schemes is not
> currently specified, and such schemes SHOULD NOT be used in the
> absence of a document specifying them" was added."

`rfc/full/rfc7296.txt:1226-1229`.

**The level is MUST, in both rows.** There is no MAY reading available. `3.6-2` says "MUST
be capable of being configured"; `3.6-3` says "MUST support". The SHOULD NOT at `:5280-5282`
governs OTHER schemes, and it is the basis of the scheme allowlist below.

### What the two formats are

> "Hash and URL of X.509 certificate    12
> Hash and URL of X.509 bundle         13"

`rfc/full/rfc7296.txt:5194-5195`. The bundle's ASN.1 is `CertBundle` at `:5241-5262`,
a `SEQUENCE OF CertificateOrCRL`.

The payload contents:

> "o  Hash and URL encodings allow IKE messages to remain short by
>    replacing long data structures with a 20-octet SHA-1 hash (see
>    [FIPS.180-4.2012]) of the replaced value followed by a variable-
>    length URL that resolves to the DER-encoded data structure itself."

`rfc/full/rfc7296.txt:5229-5232`.

### What Ze does today

| Property | Evidence | `file:line` |
|----------|----------|-------------|
| Encoding 12 has a constant | `CertEncodingHashURL uint8 = 12` | `internal/component/ike/wire/payload_cert.go` |
| That constant has NO non-test referent | grep over `internal/component/ike/` excluding `_test.go` returns only the declaration | `payload_cert.go` |
| Encoding 13 has no constant at all | the const block holds two values, 4 and 12 | `payload_cert.go` |
| Ze SENDS only encoding 4 | `buildCertPayloads` | `internal/component/ike/engine/auth.go`, `:625` |
| Ze DISCARDS any received payload whose encoding is not 4 | both collection loops | `fsm.go`, `responder.go` |
| `HTTP_CERT_LOOKUP_SUPPORTED` (16392) is not implemented | grep for `HTTPCertLookup` / `16392` over `internal/component/ike/` returns nothing | -- |
| Ze performs no outbound HTTP for certificates | no fetch site exists | -- |

**Verdict: absent, both rows.** The received-payload filter at `fsm.go` is a genuine
fail-closed guard -- a hash-and-URL payload is dropped rather than misparsed as DER -- so
today's behaviour is safe, and it is not conformant.

### The security analysis, and the finding that shapes the design

**If Ze implements the accept half literally, Ze fetches a URL an unauthenticated peer
chose.** The CERT payload arrives inside IKE_AUTH. At that moment the peer is NOT yet
authenticated -- fetching the certificate is a PRECONDITION of authenticating it. So the
fetch is a pre-authentication, attacker-controlled outbound request: a textbook SSRF, with
the daemon's network position, aimed at whatever the URL names.

**The decisive finding: the row can be satisfied WITHOUT implementing the fetch.**

Three facts, each quoted:

1. **Sending hash-and-URL requires no fetch by Ze.** Ze publishes its own certificate at an
   operator-configured URL and sends SHA-1 plus that URL. The PEER fetches.

2. **A peer may only send Ze a hash-and-URL if Ze asked for it.** RFC 7296 gates this on a
   notification Ze does not implement:

   > "The Hash and
   > URL formats of the Certificate payloads should be used in case the
   > peer has indicated an ability to retrieve this information from
   > elsewhere using an HTTP_CERT_LOOKUP_SUPPORTED Notify payload."

   `rfc/full/rfc7296.txt:5140-5143`. And:

   > "The HTTP_CERT_LOOKUP_SUPPORTED notification MAY be included in any
   > message that can include a CERTREQ payload and indicates that the
   > sender is capable of looking up certificates based on an HTTP-based
   > URL (and hence presumably would prefer to receive certificate
   > specifications in that format)."

   `rfc/full/rfc7296.txt:5395-5399`.

3. **The obligation's verb is "be capable of being configured".** A configuration leaf
   whose default is disabled satisfies "capable of being configured to accept". The
   conformant state is reachable by an operator who chooses it; it need not be the default.

**Therefore the design is:**

| Half | Implement? | Fetch? |
|------|-----------|--------|
| Send hash-and-URL (encodings 12 and 13) | yes | no. Ze emits SHA-1 + configured URL |
| Advertise `HTTP_CERT_LOOKUP_SUPPORTED` | yes, gated on the config leaf, default OFF | no |
| Accept and RESOLVE a received hash-and-URL | yes, gated on the SAME leaf, default OFF | **yes, and only here** |

With the leaf off -- the default -- Ze never advertises the notification, so a conforming
peer never sends a hash-and-URL, and the existing `fsm.go` filter continues to drop a
non-conforming one. **The SSRF surface does not exist in the default configuration.**

**Do not let this become an argument for skipping the fetch.** `ai/rules/rfc-compliance.md`
is explicit that full compliance plus full proof is the answer, and `3.6-3`'s "MUST support
the http: scheme for hash-and-URL lookup" is about the LOOKUP, which is the fetch. A
send-only implementation leaves `3.6-3` unmet. The finding above changes the DEFAULT and the
blast radius; it does not remove the obligation. Section 11 puts the question to the owner
in the form `ai/rules/rfc-compliance.md` requires.

### The bound, if the fetch is built

Every one of these is mandatory, and each has a stated reason. `ai/rules/evidence.md`
applies to all of them: on a miss, deny.

| Control | Bound | Why |
|---------|-------|-----|
| **Scheme allowlist** | `http` only. `https` MAY be added; every other scheme is refused before any DNS lookup | `:5279-5282` makes http the MUST and says other schemes "SHOULD NOT be used". `file:`, `gopher:`, `ftp:` are the classic SSRF escalations |
| **Response size** | Hard cap, 64 KiB, enforced with `io.LimitReader` and a check that the limit was not reached | A certificate is a few KiB; the bundle is a `SEQUENCE OF` and could be larger. An unbounded read against an attacker-run server is a memory exhaustion primitive |
| **Total timeout** | 5 s, on a `context.WithTimeout` covering connect, TLS, headers and body | The fetch blocks IKE_AUTH. An unbounded fetch is a per-SA thread and memory hold, reachable pre-authentication |
| **Redirect count** | 0. Set `CheckRedirect` to return an error | Redirects are the standard scheme-and-host laundering step. The hash pins the CONTENT, never the location, so following a redirect gains nothing and re-opens the host allowlist |
| **Destination allowlist** | An operator-configured host or CIDR allowlist, and by default DENY the loopback, link-local, and RFC 1918 ranges plus the cloud metadata address `169.254.169.254` | The daemon runs on a router with routes an internet host does not have. Without this the fetch is an internal port scanner |
| **Hash verification** | SHA-1 of the fetched bytes MUST equal the payload's 20 octets, checked BEFORE the bytes reach `x509.ParseCertificate` | `:5229-5232` makes the hash the identity of the fetched object. Parsing first would expose the X.509 parser to arbitrary attacker bytes with no integrity check |
| **Concurrency** | One in-flight fetch per SA, and a global cap | Otherwise N half-open SAs is N outbound connections, which is a reflection amplifier |
| **Caching** | Cache by hash, never by URL | `:5233-5234` names caching as the point of the feature. Keying on the hash makes the cache content-addressed and immune to a URL that changes what it serves |
| **No fetch when disabled** | The leaf gates the code path, not just the advertisement | A guard that only suppresses the notification still fetches for a non-conforming peer that sends the payload anyway |

**Additional non-obvious hazard, from the RFC itself:**

> "Implementations and configuration need to keep in
> mind, however, that if the URL lookups are possible only after the
> Child SA is established, recursion issues could prevent this
> technique from working."

`rfc/full/rfc7296.txt:1317-1320`. If the route to the URL runs over the tunnel being
established, the fetch deadlocks the handshake it is part of. The timeout bounds it; the
doctor check below is what tells an operator why it failed.

**Doctor check.** The spec requires one (the rfcgate-1b RFC 7296 pilot spec): the
hash-and-URL fetch is an outbound network dependency. Register it from the owning package
per `ai/rules/repo-maintenance.md`, with a `doctor-ike-cert-url-*` code in
`internal/core/diagnostic/codes.go`. It should verify the configured local publication URL
resolves, and warn when the allowlist would deny it.

### Production files

| File | Change |
|------|--------|
| `internal/component/ike/wire/payload_cert.go` | Add `CertEncodingHashURLBundle uint8 = 13`. `PayloadCERT` already carries an opaque `CertData`, so no codec change is needed: the 20-octet hash plus URL is just `CertData` |
| `internal/component/ike/wire/payload_notify.go` | Add `NotifyHTTPCertLookupSupported uint16 = 16392` |
| `internal/component/ike/engine/auth.go` | `buildCertPayloads` emits encoding 12 or 13 when configured; `buildAuthRequest` and the responder's CERTREQ path add the notification when the leaf is on |
| `internal/component/ike/engine/certurl.go` (new) | The bounded fetcher. A new file because `auth.go` is already 31 KB and this is a distinct concern (`ai/rules/go-standards.md`) |
| `internal/component/ike/engine/fsm.go`, `responder.go` | Accept encodings 12 and 13 when the leaf is on. **Both sites. Two producers.** |
| `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` | The leaf set below |
| `internal/core/diagnostic/codes.go` | The doctor codes |

**Config leaves**, under the peer `authentication` container (`ze-ipsec-conf.yang`):

| Leaf | Type | Default | Meaning |
|------|------|---------|---------|
| `hash-and-url` | `boolean` | `false` | Send hash-and-URL CERT payloads, advertise HTTP_CERT_LOOKUP_SUPPORTED, and resolve a received hash-and-URL |
| `certificate-url` | `string` | absent | The http URL at which Ze's own certificate is published |
| `certificate-url-allow` | `leaf-list` of `zt:ip-prefix` | absent | Destinations a received URL may resolve to. Empty means the built-in deny-private default only |

`ai/rules/config.md` puts all three in YANG rather than env vars: an operator would
change them during normal deployment, they need validation and commit/rollback, and they
belong in `show configuration`.

### The tagged tests

**`3.6-2` positive -- `TestHashAndURLBothFormatsAreConfigurable`.**
With `hash-and-url true`, assert `buildCertPayloads` emits a CERT payload whose encoding is
12 and whose `CertData` is exactly 20 octets of SHA-1 over the DER, followed by the
configured URL. Repeat for the bundle form, encoding 13. Assert the SHA-1 matches the DER
Ze would otherwise have sent, so the hash is over the right object.

**`3.6-2` negative -- `TestHashAndURLIsOffByDefault`.**
With the leaf absent, assert `buildCertPayloads` emits encoding 4 only, that no
HTTP_CERT_LOOKUP_SUPPORTED notification appears in any built message, and that a RECEIVED
encoding-12 payload is dropped by both collection loops. This is the security assertion and
the anti-vacuity assertion at once: it proves the positive's behaviour is caused by config,
and it pins the safe default so a later change cannot silently open the fetch.

**`3.6-3` positive -- `TestHashURLLookupUsesHTTPAndVerifiesTheHash`.**
Against an `httptest.Server`: a received encoding-12 payload naming that server's http URL
is fetched, the SHA-1 is verified, and the resulting certificate authenticates the peer.
This is the `3.6-3` MUST -- the http scheme is supported for lookup.

**`3.6-3` negative -- `TestHashURLLookupRefusesEverythingOutsideTheBound`.**
Table-driven, one row per control in the bound table. Each MUST be its own row, because a
single "bad URL is refused" row can pass while five of the seven controls are missing:

| Row | Fixture | Assert |
|-----|---------|--------|
| scheme | `file:///etc/passwd` | refused before any I/O |
| scheme | `ftp://host/x` | refused |
| size | server sends 1 MiB | refused, and no more than the cap is read |
| timeout | server sleeps past the deadline | refused within the bound |
| redirect | server 302s to a second server | refused, and the second server records zero requests |
| destination | URL naming `169.254.169.254` | refused before connect |
| hash | server returns a VALID but DIFFERENT certificate | refused, and `x509.ParseCertificate` was never reached |

The last row is the most important one in the package. A fetcher that parses first and
compares the hash afterwards passes every other row and is still exploitable.

### Mutations

| Mutation | Site | Must redden |
|----------|------|-------------|
| The scheme check accepts any scheme | `certurl.go` | the negative's two scheme rows |
| `io.LimitReader` cap is removed | `certurl.go` | the size row |
| `CheckRedirect` is left at the Go default | `certurl.go` | the redirect row |
| The hash comparison moves to after the parse | `certurl.go` | the hash row's "parser never reached" assertion ONLY. Run it |
| The private-range deny is dropped | `certurl.go` | the destination row |
| The leaf gate is applied to the advertisement only, not the fetch path | `fsm.go` | `3.6-2`'s negative, the received-payload half |
| Encoding 13 is accepted at `fsm.go` but not at `responder.go` | one site only | `3.6-2` positive on the responder path ONLY. **Mutate both sites separately** |

---

## 8. `RFC7296-2.15-3` -- hex encoding of the shared secret

### The obligation, verbatim

> "The
> management interface by which the shared secret is provided MUST
> accept ASCII strings of at least 64 octets and MUST NOT add a null
> terminator before using them as shared secrets.  It MUST also accept
> a hex encoding of the shared secret.  The management interface MAY
> accept other encodings if the algorithm for translating the encoding
> to a binary string is specified."

`rfc/full/rfc7296.txt:2876-2881`. `2.15-1` is `:2876-2877`; `2.15-2` is `:2877-2878`;
`2.15-3` is `:2878-2879`. The MAY at `:2879-2881` is not a row and must not be implemented
as part of this one.

### The constraint the two sibling rows impose

`RFC7296-2.15-1` and `-2` are committed and proven:

| Row | Positive | Negative |
|-----|----------|----------|
| `2.15-1` | `internal/component/ike/ipsec/rfc7296_test.go` | `:37` |
| `2.15-2` | `internal/component/ike/ipsec/rfc7296_test.go` | `:84` |

`TestPSKAcceptsAtLeast64ASCIIOctets` (`rfc7296_test.go`) asserts that the FULL
printable-ASCII range 0x20..0x7e survives the parse byte for byte.

**That test is the design constraint for this row, and it is why autodetection is banned.**
An ASCII secret of `"abcdef0123456789"` is valid printable ASCII, valid under `2.15-1`, and
also a valid hex string. Any implementation that guesses the encoding from the value would
silently reinterpret that operator's secret as eight binary octets, break an established
deployment, and violate `2.15-1` at the same time. The spec named this risk directly:
"Hex PSK decoding (`2.15-3`) must not change how an existing ASCII PSK is interpreted"
(the rfcgate-1b RFC 7296 pilot spec).

### What Ze does today

| Property | Producing function | `file:line` |
|----------|--------------------|-------------|
| The management interface | `parseAuthConfig` | `internal/component/ike/ipsec/config.go` |
| The PSK leaf is read and assigned through | `parseAuthConfig` | `config.go` |
| An existing at-rest encoding IS already handled | `parseAuthConfig` | `config.go`, via `secret.IsEncoded` / `secret.Decode` |
| That mechanism uses an EXPLICIT PREFIX MARKER | `IsEncoded` | `internal/component/config/secret/secret.go`, marker `"$9$"` at `:21-22` |
| The YANG leaf | leaf `pre-shared-secret` | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` |
| The secret is used as raw octets | `computePSKAuth` | `internal/component/ike/engine/auth.go` (`[]byte(psk)` into `ikecrypto.PRF`) |
| No hex decoding anywhere in the IKE tree | grep for `hex.Decode` over `internal/component/ike/` | none |

**The precedent is already in the function that needs changing.** `secret.IsEncoded` is an
explicit-marker test, not a heuristic, and it sits four lines from where the hex support
goes.

### Production design

**Layer: `internal/component/ike/ipsec/config.go`, `parseAuthConfig`.** This is the
MANAGEMENT INTERFACE, which is the subject of the obligation ("The management interface by
which the shared secret is provided MUST ..."). It is also where `2.15-1` and `-2` are
proven, so all three rows end up with one producer and one test home.

**Not `computePSKAuth`.** Decoding at AUTH time would leave the parsed `AuthConfig.PSK`
holding hex text, so `show configuration`, the config diff, and every other reader would see
a different value from the one the PRF consumes. The obligation is on the interface; decode
at the interface.

**The selector.** Two shapes, and the first is recommended:

| Option | Shape | Assessment |
|--------|-------|------------|
| **A. Sibling enum leaf** | `leaf pre-shared-secret-encoding { type enumeration { enum ascii; enum hex; } default ascii; }` | **Recommended.** Explicit, visible in `show configuration`, diffable, and impossible to collide with a value. `ai/rules/config.md` favours it |
| B. Value prefix, e.g. `0x` | mirrors `secret.Prefix` | Rejected. `0x...` is itself a legal ASCII secret, so it re-introduces the collision the marker was meant to remove. `$9$` is safe only because it is already reserved |

With option A, `parseAuthConfig` reads the encoding leaf and, when `hex`, runs
`hex.DecodeString` after the existing `secret.Decode` step, so an operator may still store a
hex secret obfuscated at rest. Order matters: `$9$` unwrapping first, hex second.

**Fail closed on a malformed value.** `hex.DecodeString` errors on an odd length or a
non-hex digit. Return the error and refuse the config, per `ai/rules/protocol.md` --
never fall back to treating the value as ASCII. A silent fallback would make a typo in a hex
secret produce a working-looking config that authenticates against nothing, and the operator
would debug the peer instead of the typo.

**Naming.** `pre-shared-secret-encoding` mirrors the leaf it modifies, is kebab-case, and
carries no abbreviation, per `ai/rules/config.md`. If a matching env var is added it
must be `ze.ike.<path>.pre-shared-secret-encoding`.

### The tagged pair

Add to `internal/component/ike/ipsec/rfc7296_test.go`, beside `2.15-1` and `-2`. The spec's
phase list names the test `TestHexPSKDecoding`; prefer a name matching the file's
existing convention and record the rename.

**Positive -- `TestPSKAcceptsHexEncoding`.**
With `pre-shared-secret-encoding hex` and a value of `"0a1b2c..."`, assert the parsed
`AuthConfig.PSK` holds the DECODED octets, byte for byte, and has half the input's length.
Include a value whose decoded form contains a NUL and a 0xFF, which proves the result is a
binary octet string rather than text: `2.15-2` guarantees no terminator is added, and a hex
secret is exactly where a NUL legitimately appears mid-value.

Add the end-to-end half: assert `computePSKAuth` over the decoded secret produces the same
AUTH as an ASCII-mode peer whose literal secret is those raw octets. That proves the decode
reaches the PRF and is not merely stored, which no parse-only assertion can show.

**Negative -- `TestHexEncodingIsExplicitAndNeverGuessed`.**
Three assertions, and the first is the one that protects existing deployments:

1. **The collision case.** The value `"abcdef0123456789"` with the encoding leaf ABSENT (or
   `ascii`) parses to those sixteen ASCII characters, unchanged. This is the regression
   guard for every deployed hex-looking ASCII secret, and it is the assertion that makes
   autodetection impossible to introduce later without a red test.
2. **Malformed hex is refused.** `"abc"` (odd length) and `"zz"` (non-hex) with
   `encoding hex` return an error from `ParseIPsecConfig`. Assert the error names the peer
   and the leaf.
3. **No silent fallback.** After the refusal in step 2, assert the peer is absent from the
   parsed config rather than present with an ASCII-interpreted secret.

### Mutations

| Mutation | Site | Must redden |
|----------|------|-------------|
| The hex branch is applied whenever the value parses as hex | `parseAuthConfig` | the negative's step 1. This is THE mutation the package exists to prevent -- run it explicitly |
| `hex.DecodeString`'s error is discarded and the raw string is kept | `parseAuthConfig` | the negative's steps 2 and 3 |
| The decode runs before `secret.Decode` | `parseAuthConfig` | a `$9$`-wrapped hex secret fails. Add one positive row covering the combination so this ordering is pinned |
| The decoded value is stored but `computePSKAuth` re-encodes it | `auth.go` | the positive's end-to-end half ONLY. Run it: the parse-only assertion cannot see this |
| The encoding leaf default changes from `ascii` to `hex` | `ze-ipsec-conf.yang` | the negative's step 1, and `TestPSKAcceptsAtLeast64ASCIIOctets` (`rfc7296_test.go`) |

---

## 9. Id allocation

`check_id_allocation` (`scripts/dev/rfc_requirements.py`, refusal at `:503`) refuses a
new id whose ordinal is at or below its section's high-water mark. The mark is computed from
the COMMITTED HEAD summary by `_git_baseline_ids`, which runs
`git show HEAD:<path>` -- not the index, not the working tree. **A section with no committed
id is skipped entirely** (`:500-502`).

### Marks measured 2026-07-31, from HEAD

**A concurrent session IS editing `rfc/short/rfc7296.md` right now** (WP-5's package: the
working tree adds `RFC7296-2.5-14`, `-15`, `-16`, `-17`, `RFC7296-3.2-4` and
`RFC7296-3.12-4`). None of those ids is in a WP-10 section, so **every mark below is
unaffected by that in-flight edit**, whether it is read from HEAD or from the tree it
produces. Verified 2026-07-31 with `git diff rfc/short/rfc7296.md`. Re-verify at landing:
the fact that a §2.5 block is landing today is exactly the reason section 9's contiguity
rule exists for §4.

Commands:

    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-3\.6-[0-9]*'   | sort -V | tail -1
    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-3\.5-[0-9]*'   | sort -V | tail -1
    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-3\.3\.4-[0-9]*'| sort -V | tail -1
    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-4-[0-9]*'      | sort -V | tail -1
    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-2\.15-[0-9]*'  | sort -V | tail -1

| Section | Committed ids at HEAD | Mark | Appendix A ordinal | Verdict |
|---------|----------------------|------|--------------------|---------|
| `RFC7296-3.5` | none | **none** | `3.5-2`, `-3`, `-4` | **accepted as-is.** Section skipped |
| `RFC7296-3.6` | none | **none** | `3.6-1`, `-2`, `-3` | **accepted as-is.** Section skipped |
| `RFC7296-4` | none | **none** | `4-4` | **accepted as-is.** Section skipped |
| `RFC7296-2.15` | -1, -2 | **2** | `2.15-3` | **accepted as-is.** 3 > 2 |
| `RFC7296-3.3.4` | -2, -3 | **3** | `3.3.4-1` | **REFUSED.** Must land as `3.3.4-4` or higher |

`_head_of` keys on the section STRING (`scripts/dev/rfc_requirements.py`), so
`RFC7296-3.5` and `RFC7296-3.5.x` are distinct scopes, as are `RFC7296-4` and
`RFC7296-4.1`. They do not share marks.

### `RFC7296-3.3.4-1` renumbers to `RFC7296-3.3.4-4`

This is the package's one forced renumbering, and it produces a readable oddity: the
management-facility sentence is the FIRST of the three in §3.3.4, and its row will sit LAST
in the checklist, after the two sentences that depend on it. That is unavoidable -- `-2` and
`-3` were committed first -- and it is worth one line in the row text so a reader is not
confused. Correct Appendix A in the same commit, per the precedent set for `1.4-2` to
`1.4-5` (the rfcgate-1b RFC 7296 pilot spec).

### The contiguity warning: §3.6 and §4

**§3.6 is safe.** Appendix A holds exactly three §3.6 rows and all three are WP-10's. No
other package can move the mark first. Land `3.6-1`, `-2`, `-3` together and the block is
contiguous from 1.

**§4 is NOT safe, and this is the trap.** Appendix A holds four §4 rows and WP-10 owns only
the last:

| Appendix A id | Owner | Spec citation |
|---------------|-------|---------------|
| `RFC7296-4-1` | WP-14 (SA lifecycle) or unassigned | `:1244` |
| `RFC7296-4-2` | **WP-9** (configuration payload, remote access) | `:1245`, phase list `:725` |
| `RFC7296-4-3` | **WP-9** | `:1246`, phase list `:725` |
| `RFC7296-4-4` | **WP-10** | `:1247` |

Section 4 has no committed id, so the FIRST package to land in §4 sets the mark. If WP-10
lands `4-4` alone, the mark jumps to 4 and ordinals 1, 2 and 3 become **permanently
unusable** by WP-9 and WP-14. Three ids stranded, for nothing.

**The rule: whichever package lands first in §4 takes a contiguous block starting at 1, in
Appendix A ordinal order.**

| Landing order | `4-4` becomes |
|---------------|---------------|
| All four together | `4-4` |
| WP-10 alone, first | **`4-1`** |
| WP-9 first (taking `4-1`, `4-2`), then WP-10 | **`4-3`** |

**Recompute at the moment of landing. Never hardcode.** Three renumberings have already
cost this spec time.

WP-10 is scheduled at phase item 14 and WP-9 at item 15, so on the phase
list WP-10 lands FIRST and would take `4-1`. But the 2026-07-30 re-triage renumbered every
package, so the landing order is genuinely open. Do not assume it.

### If `RFC7296-3.3.4-1` moves to WP-9

The re-triage's WP-9 is "Crypto suite policy and management facility" (`:1573`), which is
this row's subject matter almost word for word. If the owner routes it there, WP-10 drops to
eight rows -- matching the re-triage's count of 8 and resolving the discrepancy
noted at the top of this document. That is the most likely explanation of the mismatch, and
section 11 raises it. This design's section 4 travels with the row wherever it goes.

---

## 10. What must NOT break

| Invariant | Why it is at risk | The guard |
|-----------|-------------------|-----------|
| **`RFC7296-1.2-2`, send side.** The first CERT payload holds the AUTH key | Section 5 rewrites `buildCertPayloads` to emit up to four payloads. Appending the leaf after a loop over intermediates would invert the order | `TestAutFirstCertificateCarriesTheAuthKey` (`internal/component/ike/engine/rfc7296_auth_test.go`, tags at `:250-253`) reddens. **Its tag cites `auth.go` for `buildCertPayloads`, which is now at `auth.go`.** Correct the comment in the same commit -- and note that this is a tagged block, so `.claude/hooks/pretool-writeedit.py`'s `rfc-tagged-test` check applies. A comment-only edit passes it; do not touch the assertions |
| **`RFC7296-1.2-2`, receive side.** The FIRST CERT payload is the peer certificate and later ones are intermediates | Section 5 adds a cap to `storeRemoteCerts`, the exact function this proves | `TestRccTwoLevelChainAuthenticates` (`internal/component/ike/engine/remote_cert_chain_test.go`, tags at `:210-215`) reddens. It asserts BOTH orders on BOTH roles, so a cap that reorders or drops is caught four ways |
| **`RFC7296-2.15-1`.** The interface accepts 64+ ASCII octets, full printable range | Section 8 changes the PSK parse path | `TestPSKAcceptsAtLeast64ASCIIOctets` (`internal/component/ike/ipsec/rfc7296_test.go`) reddens, specifically its printable-ASCII sweep at `:57-69` |
| **`RFC7296-2.15-2`.** No null terminator added, none stripped | Same path | `TestPSKHasNoNullTerminatorAdded` (`rfc7296_test.go`) reddens |
| **`RFC7296-3.3.4-2` and `-3`.** Transform IDs compared against local policy; unauthorized proposals rejected | Section 4 touches no code, but a YANG change to `list proposal` while adding the auth leaves could disturb the parse | `TestPropTransformIDsComparedAgainstLocalPolicy` (`internal/component/ike/crypto/rfc7296_proposal_test.go`) and `TestPropUnauthorizedProposalRejected` both redden |
| **The two-level chain deployment.** A leaf signed by an intermediate authenticates | Section 5 changes `CertificateEntry.RawInter` from a scalar to a slice, and four non-test readers depend on it | `TestRccMissingIntermediateNamesTheChain` (`remote_cert_chain_test.go`). Audit `internal/component/pki/show.go`, `:197-198`, `internal/component/pki/store.go` in the same commit |
| **`TestRccSentCertPayloadsCarryTheIntermediate` asserts EXACTLY TWO payloads** | It calls `buildCertPayloads(sa)` DIRECTLY (`remote_cert_chain_test.go`, call at `:274`) and asserts the count. Section 5 raises the count to four | **This test WILL redden, by design, and it is the one place the implementer must edit an existing assertion.** It carries NO `RFC requirement:` tag, so `rfc-tagged-test` does not gate it -- but `c_test_weakening` does. Do not relax the count to `>= 1`. Change it to assert the exact number the fixture configures, so it keeps discriminating |
| **The remote-id policy gate.** Section 6 adds arms to `certificateCarriesIdentity` and `assertedIdentity` | Every existing remote-id test | `internal/component/ike/engine/remote_id_test.go`, twelve test functions from `:89` to `:425`. `TestRidDeniesAnIdentityTypeItCannotCompare` asserts a DN DENIES and is the direct contradiction of section 6 -- it must be rewritten to assert the new contract, not deleted. Also `remote_cert_trust_test.go`, `:138`, `remote_cert_eku_test.go`. Run the whole engine package |
| **`encodeIKEID`'s direct test table** | Section 1 adds tests beside it; a later implementer may be tempted to extend `encodeIKEID` for section 6's types | `responder_test.go` holds a table over `encodeIKEID` type and length. Section 1's tests must not duplicate it -- extend that table or cite it |
| **`ValidateIdentities` and its three tests** | Section 6 lifts the DN refusal | `internal/component/ike/ipsec/validate_identity_test.go`, `:60`, and `internal/component/ike/engine/config_verify_test.go`. Rewrite each to the new contract in the same commit |
| **The unset-`remote-id` warning stays.** `auth.go` | Section 6 restructures the identity binding around it | Section 6's negative half 2 pins the warning text. Without it, the fail-open path could go silent |
| **`test/ipsec/` `.ci` suite stays green** -- 8 files: `ipsec-child-rekey`, `ipsec-clear-reestablish-peer`, `ipsec-clear-reestablish`, `ipsec-clear-sa`, `ipsec-sa-installed`, `ipsec-show-peer`, `ipsec-show-sa`, `ipsec-show-status` | Every YANG leaf this package adds enters the config those tests parse | `make ze-ipsec-test`. **The suite genuinely earns a verify tier**: `ipsec` is in `all_suites` (`mk/test-functional.mk`) AND has a `run_suite` line. A historical defect where it was listed but never ran is recorded at `mk/test-functional.mk` and is fixed. Do not re-open it |
| **`test/ipsec/` has ZERO certificate or identity coverage** | All eight `.ci` files configure `mode pre-shared-secret` with `pre-shared-secret test-key-1234` and set NO `local-id`, `remote-id`, `certificate`, or `ca-certificate` (`ipsec-sa-installed.ci` and `:144`, and the seven siblings) | **The `.ci` layer cannot guard sections 5, 6 or 7 at all.** `ai/rules/testing.md` requires a functional test per user-facing behaviour, and every YANG leaf this package adds is user-facing. WP-10 must ADD `.ci` coverage, not merely avoid breaking it: at minimum `ipsec-psk-hex` (named by the spec at `:718`), plus one certificate-auth `.ci`. The suite earns a verify tier, so this coverage is merge-gating |
| **`test/ipsec-interop/` scenarios stay green** | Sections 5, 6 and 7 change wire bytes: more CERT payloads, new encodings, a new notification. Ten scenarios exist (01-05, 07-11; there is no 06) and ALL TEN set both `local-id` and `remote-id` | `make ze-ipsec-interop-test` (`mk/test-integration.mk`); one scenario via `IPSEC_INTEROP_SCENARIO=<name>`. **Scenario 04 (EAP-TLS) is ALREADY FAILING for an unrelated reason**, owned by `spec-fixit-eap-tls-clienthello-race`. Record its failure SIGNATURE before the package starts and compare signatures, not pass/fail. Do not adopt that failure and do not let it mask a new one |
| **Scenario 04's fixture is the CN-fallback path** | `test/ipsec-interop/scenarios/04-eap-tls/ze.conf` sets `local-id "ze-test-client"` and `remote-id "172.28.0.3"`, and `gen-pki.sh` mints `client.pem` with CN `ze-test-client` and **no SAN extension**, beside `server.pem` which DOES carry `subjectAltName=IP:172.28.0.3` | Section 6 changes `certificateCarriesIdentity`, whose CN fallback is conditional on `hasSubjectAltName` (`remote_id.go`). Scenarios 03, 04 and 08 are the only cert-bearing ones, and all three ride this fixture. `TestRidAcceptsACommonNameWhenNoSubjectAltNameExists` (`remote_id_test.go`) is its unit-test mirror |
| **The interop PKI plumbing refuses a multi-certificate PEM** | Section 5 needs interop fixtures with more intermediates. `lab.py` REFUSES a two-certificate bundle and a certificate beside a key, deliberately and fail-closed | Adding intermediates to an interop scenario means one file per certificate and one `%%PKI_B64:<file>%%` placeholder each (`lab.py`, `:575-632`), never a concatenated bundle. `test/ipsec-interop/lab_test.py` `test_every_pki_scenario_resolves_to_decodable_der` iterates every scenario's `ze.conf` and would catch a malformed one -- **but `lab_test.py` is wired to no make target**, so it will not run unless invoked directly. Say so rather than relying on it |
| **strongSwan interop on identity types** | `encodeIKEID`'s doc records that strongSwan rejects an IP value sent as ID_FQDN with "constraint check failed" (`auth.go`). Section 6's `remote-id-type` leaf must not change the default SEND behaviour | The interop suite. Section 1's tests pin the send-side type selection unchanged |
| **A conformant peer never triggers a fetch by default** | Section 7 | `3.6-2`'s negative asserts no HTTP_CERT_LOOKUP_SUPPORTED is advertised with the leaf off, and that a received encoding-12 payload is dropped |
| **`ai/RFC-REQUIREMENTS.md` stays fresh** | Nine new tagged tests move line numbers throughout | Run `make ze-rfc-index` and commit the ledger in the SAME commit. `ze-rfc-check` fails on a stale ledger in both verify modes (`ai/rules/testing.md`, RFC-Tagged Tests) |

---

## 11. Risks

| Id | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-WP10-1 | **The hash-and-URL fetch ships as an unbounded SSRF.** The natural implementation is `http.Get(url)`, which follows up to 10 redirects, reads without limit, has no timeout, and accepts any scheme the URL names. Every one of those is a control in section 7 | none from any existing test; a bounded and an unbounded fetcher both make the positive green | Section 7's negative is table-driven with ONE ROW PER CONTROL, and the mutation table names the site for each. The row that matters most is hash-before-parse. Treat a missing row as a missing control |
| R-WP10-2 | **Hex PSK autodetection is introduced and silently breaks a deployed ASCII secret** that happens to be hex-shaped. It also violates `2.15-1`, which is already green | `TestPSKAcceptsAtLeast64ASCIIOctets` may stay green: its fixtures are letters and full printable ASCII, not a pure hex string | Section 8's negative step 1 asserts `"abcdef0123456789"` parses as ASCII with the leaf absent. Run the autodetect mutation explicitly; it is the one this row exists to prevent |
| R-WP10-3 | **The certificate cap truncates instead of refusing.** Truncation looks like a bound, passes a count assertion, and hides from the operator that a limit was hit | none. A truncating implementation passes any count-based test | Section 5's negative step 3 asserts the chain is EMPTY and the SA is not established after a refusal. Run the truncate mutation |
| R-WP10-4 | **The accept cap lands at one of two collection sites.** `fsm.go` and `responder.go` both call `storeRemoteCerts`, and both must handle its new error | a single-path test stays green | `storeRemoteCerts` is the single funnel, so the cap itself is one site. The ERROR HANDLING is two sites. Mutate each independently, per section 5's table. This is the second-producer shape the spec has recorded three times |
| R-WP10-5 | **`4-4` is classified partial without an owner ruling, or is "fixed" by pointing at the unset-`remote-id` path.** Accepting via an absent check is acceptance by fail-open, and `ai/rules/rfc-compliance.md` voids prior answers that point away from full compliance | none mechanical | Section 12. Raise before landing. Section 6's negative half 2 pins the warning so the fail-open path stays visible |
| R-WP10-6 | **Four rows are classified conformant against an Appendix A that says NOT IMPL.** Four such reclassifications in one package invites a reviewer to suspect the package rather than the classification | a reviewer challenges the whole set | Each of sections 1 to 4 cites the producing function and, for `3.3.4-1`, two ALREADY-GREEN dependent tests. Present the evidence per row, never as a block. Note the precedent: the 2026-07-30 triage found the walk "wrong in both directions, for the third time" (`:1594-1598`) |
| R-WP10-7 | **`4-4`'s Appendix A text elides the ID-type clause**, so a reader working from the row alone classifies it conformant and closes it | none; the row reads as satisfied | Section 6 restores the clause. Fix the row text when it lands, and read `rfc/full/rfc7296.txt:6895-6910` for the third bullet the walk did not capture |
| R-WP10-8 | **§4 ordinals are stranded.** WP-10 lands `4-4` alone, the mark jumps to 4, and WP-9's `4-2`/`4-3` become unusable | `check_id_allocation` fails on WP-9's commit, weeks later, in someone else's session | Section 9's contiguous-block rule. Recompute the mark at landing. Coordinate with WP-9, which owns two of the four §4 rows |
| R-WP10-9 | **The `remote-id-type` leaf changes the SEND path.** Section 6 introduces it for the accept side; an implementer may wire it into `encodeIKEID` too, changing which type Ze sends and breaking strongSwan interop | the interop suite reddens, but only in the nightly tier | Section 1's positive pins `encodeIKEID`'s behaviour against `local-id` alone. Keep the leaf accept-side only, or add a separate `local-id-type` -- do not overload one leaf across both directions |
| R-WP10-10 | **Interop scenario 04 masks a real regression.** EAP-TLS is failing before this package starts (`spec-fixit-eap-tls-clienthello-race`) | a WP-10 change makes 04 fail differently and nobody notices | Capture the scenario-04 failure signature BEFORE any edit. Compare signatures, not pass/fail. `ai/rules/completion.md`: a pre-existing red is not a licence to stop looking |
| R-WP10-11 | **`3.6-3` is quietly satisfied by the send half alone.** "Support the http: scheme for hash-and-URL LOOKUP" is about fetching. A send-only implementation leaves it unmet while `3.6-2` goes green | `3.6-2`'s tests pass and `3.6-3` has no test that performs a lookup | Section 7's `3.6-3` positive requires a real `httptest.Server` fetch. If that test is not written, `3.6-3` is not proven, whatever `3.6-2` shows |
| R-WP10-12 | **Engine line numbers move.** Other agents are editing `internal/component/ike/engine/` now; `auth.go` and `responder.go` both changed today | a tag cites a line holding different code | Every citation here names its function. Re-locate by function name and re-read before quoting a line in a tag. `rfc7296_auth_test.go` is already a live example of a tag with a stale locator |
| R-WP10-13 | **A new config leaf is INERT ON RELOAD.** `peersEqual` (`internal/component/ike/ipsec/types.go`) compares six auth fields by name. A seventh that is not added there means an operator's edit commits and changes nothing | none. The config shows the new value, `show configuration` agrees, and the session never renegotiates | Add EVERY new leaf to `peersEqual` in the same commit, and write one reload test per leaf. This is the shape `ai/rules/completion.md` calls an inert config surface, and it is invisible to every test in this design that parses config without reloading |
| R-WP10-14 | **`ValidateIdentities`' DN refusal is lifted before the engine can handle a DN**, leaving a config that commits and an SA that dies at IKE_AUTH | section 6's positive sub-test 3 fails at the daemon rather than at parse, which reads as a test bug | Lift `validate.go` in the SAME commit as the `certificateCarriesIdentity` arm, never before. Rewrite its three tests to the new contract rather than deleting them (`ai/rules/testing.md`) |
| R-WP10-15 | **The `.ci` layer is silently skipped.** `ze-functional-test` honours `ZE_SKIP_SUITES` (`mk/test-functional.mk`, checked at `:195` and `:199-201`), and the ipsec suite is a documented skip candidate in Docker-constrained environments | a green functional run that never executed a single ipsec test | Assert the suite ran, not that the run was green. When claiming `.ci` evidence for this package, paste the ipsec suite's own result line. The suite once sat in `all_suites` with no `run_suite` line and was credited a tier it did not earn (`mk/test-functional.mk`) -- the same class of false evidence |
| R-WP10-16 | **The `dh-group` gate fix is dropped as out of scope.** It surfaced while establishing `3.3.4-1`, not from the row text | none; `3.3.4-1` can be classified conformant without it | `ai/rules/completion.md`: finding a defect while doing something else is the reason you are the one who fixes it. It is a one-predicate change mirroring `config.go`. Section 4's negative row 3 is its test. If the owner rules it out of scope, it needs a named destination spec, not a note |

---

## 12. Items for the owner

`ai/rules/rfc-compliance.md` requires that anything short of full compliance plus full proof
goes to Thomas, quoted, with the producing `file:line` and the cost, asked as "which way do
you want it fixed" and never as "may I skip it". Three items qualify. **All three are
questions about HOW, and none proposes doing less.**

### OR-WP10-A: `3.6-2` / `3.6-3` -- build the fetch, or ship send-only and keep the rows open?

**The requirement.**

> "and also MUST be capable of being configured to send and accept the
> two Hash and URL formats (with HTTP URLs)."

`rfc/full/rfc7296.txt:5273-5274`.

> "Implementations MUST support the "http:" scheme for hash-and-URL
> lookup."

`rfc/full/rfc7296.txt:5279`.

**Ze today.** `CertEncodingHashURL uint8 = 12` (`internal/component/ike/wire/payload_cert.go`)
has no non-test referent; encoding 13 has no constant; `buildCertPayloads`
(`internal/component/ike/engine/auth.go`) emits encoding 4 only; both collection
loops discard anything else (`fsm.go`, `responder.go`); and
HTTP_CERT_LOOKUP_SUPPORTED (16392) is absent.

**What full compliance costs.** The send half plus the notification is roughly a day. The
accept half is the fetch, and the fetch is a pre-authentication outbound request to an
attacker-named URL. Section 7's bound lists nine controls, each of which needs its own test
row, plus a doctor check and three YANG leaves. Call it three to four days, most of it the
security bound rather than the protocol.

**The finding that shapes the question.** RFC 7296 gates a peer's use of hash-and-URL on Ze
advertising HTTP_CERT_LOOKUP_SUPPORTED, and the obligation's
verb is "capable of being CONFIGURED". So the fetch can be built behind a leaf that defaults
OFF, and the SSRF surface does not exist for any operator who does not opt in. That is the
design in section 7, and it is full compliance with a safe default -- not a reduction.

**The question.** "Section 7 designs `3.6-2` and `3.6-3` in full, with the fetch behind a
config leaf defaulting to disabled, and a nine-control security bound. Do you want it built
that way in WP-10, or split so the send half lands now and the fetch lands in its own spec
with its own review? If it splits, `3.6-3` stays OPEN and un-annotated -- it is not a
`{gap}`."

### OR-WP10-B: `4-4` -- narrow binding, or the `remote-id-type` leaf?

**The requirement.**

> "o  Public Key Infrastructure using X.509 (PKIX) Certificates
>    containing and signed by RSA keys of size 1024 or 2048 bits, where
>    the ID passed is any of ID_KEY_ID, ID_FQDN, ID_RFC822_ADDR, or
>    ID_DER_ASN1_DN."

`rfc/full/rfc7296.txt:6877-6880`.

**Ze today.** `certificateCarriesIdentity`
(`internal/component/ike/engine/remote_id.go`) has arms for ID_FQDN and
ID_RFC822_ADDR and falls to `return false` for ID_KEY_ID.
`assertedIdentity` (`remote_id.go`) reports ID_DER_ASN1_DN not comparable, so
`checkRemoteIdentity` refuses it at `:247-251`. Two of the four required types cannot be
accepted with `remote-id` set. The ID_KEY_ID refusal is a deliberate fail-closed choice
documented at `remote_id.go`.

**What full compliance costs.** ID_DER_ASN1_DN is tractable and cheap: compare the asserted
DER against `cert.RawSubject` with `bytes.Equal`, which is exact and needs no canonical form
-- so the objection recorded at `remote_id.go` does not apply to the certificate
comparison. Half a day. ID_KEY_ID cannot bind to any certificate field by construction, so
it needs an operator opt-in; section 6 proposes `remote-id-type`, which costs about a day
and also closes the fail-open widening in section 2.

**The question.** "`RFC7296-4-4` requires PKIX acceptance where the ID is ID_KEY_ID or
ID_DER_ASN1_DN, and `certificateCarriesIdentity` (`remote_id.go`) denies both. Section 6
proposes a `remote-id-type` enum leaf that satisfies the MUST as an explicit operator
decision, and as a side effect lets an operator pin ID_RFC822_ADDR against ID_FQDN, closing
a fail-open widening at `remote_id.go`. Do you want the narrow fix (DN binding plus a
key-id opt-in) or the leaf, which costs about a day more and closes both?"

State plainly, in the same message: **the unset-`remote-id` path is NOT an answer.** It
accepts those types today by having no check, and `auth.go` logs that every
certificate the authority issued authenticates as the peer.

### OR-WP10-C: `3.3.4-1`'s home, and the row-count discrepancy

Not a compliance question -- a scope question, and it is the likely explanation of the
8-versus-9 mismatch.

The phase list gives WP-10 nine rows including `RFC7296-3.3.4-1`. The re-triage
table gives WP-10 eight rows and creates WP-9 "Crypto suite policy and management
facility" (`:1573`), which is `3.3.4-1`'s subject matter almost word for word.

**The question.** "Does `RFC7296-3.3.4-1` belong to WP-10 or to the re-triage's WP-9? It is
conformant either way -- the `ike-group / proposal` YANG list
(`ze-ipsec-conf.yang`) is the management facility, and `3.3.4-2`/`-3` are already
proven against it. Routing it to WP-9 makes WP-10 eight rows and reconciles the tables. It
also removes the package's only forced renumbering, since `3.3.4-1` must become `3.3.4-4`
(section 9)."

This is a proposal about where work sits, not about doing less, so it needs no permission
under `ai/rules/completion.md`. Raise it with A and B so all three are answered together.

---

## 13. Summary row texts to add

Land them in section order in the checklist of `rfc/short/rfc7296.md`. Ordinals below assume
WP-10 lands §4 FIRST; recompute per section 9. Each row is ONE physical line in the file --
the wrapping here is for this document only, since `parse_checklist_line` reads one row per
line, and it validates that the id's section segment agrees with the `(§X.Y)` citation.

    - [ ] [RFC7296-2.15-3] [MUST] It MUST also accept a hex encoding of the shared secret (§2.15)

    - [ ] [RFC7296-3.3.4-4] [MUST] All implementations of IKEv2 MUST include a management
      facility that enables a user or system administrator to specify the suites that are
      acceptable for use with IKE (§3.3.4)

    - [ ] [RFC7296-3.5-2] [MUST] To assure maximum interoperability, implementations MUST be
      configurable to send at least one of ID_IPV4_ADDR, ID_FQDN, ID_RFC822_ADDR, or
      ID_KEY_ID (§3.5)

    - [ ] [RFC7296-3.5-3] [MUST] Implementations MUST be configurable to accept all of these
      four types (§3.5)

    - [ ] [RFC7296-3.5-4] [MUST] IPv6-capable implementations MUST additionally be
      configurable to accept ID_IPV6_ADDR (§3.5)

    - [ ] [RFC7296-3.6-1] [MUST] Implementations MUST be capable of being configured to send
      and accept up to four X.509 certificates in support of authentication (§3.6)

    - [ ] [RFC7296-3.6-2] [MUST] Implementations MUST be capable of being configured to send
      and accept the two Hash and URL formats (with HTTP URLs) (§3.6)

    - [ ] [RFC7296-3.6-3] [MUST] Implementations MUST support the "http:" scheme for
      hash-and-URL lookup (§3.6, §1.7)

    - [ ] [RFC7296-4-1] [MUST] For an implementation to be called conforming to this
      specification, it MUST be possible to configure it to accept PKIX certificates
      containing and signed by RSA keys of size 1024 or 2048 bits, where the ID passed is
      any of ID_KEY_ID, ID_FQDN, ID_RFC822_ADDR, or ID_DER_ASN1_DN, and shared key
      authentication where the ID passed is any of ID_KEY_ID, ID_FQDN, or ID_RFC822_ADDR
      (§4)

**Note the two corrections carried by these texts.** `3.3.4-1` becomes `3.3.4-4` (section 9),
and the `4-4` row restores the ID-type clause that Appendix A elided (section 6, R-WP10-7).
The `4-4` ordinal shown as `4-1` assumes WP-10 lands §4 first; if WP-9 lands first it is
`4-3`.

After the rows land, run `make ze-rfc-index` and commit `ai/RFC-REQUIREMENTS.md` in the SAME
commit (`ai/rules/testing.md`, RFC-Tagged Tests).

---

## 14. Suggested commit shape

The package splits cleanly, and landing it as one commit would mix nine rows, five
production surfaces and three YANG leaves into one review.

| Commit | Rows | Content |
|--------|------|---------|
| 1 | `3.5-2`, `3.5-3`, `3.5-4`, `3.3.4-1` | Eight tagged tests, four summary rows, plus the ONE production fix section 4 found: the `dh-group` parse gate (`DHGroupImplemented` called from `config.go`). The stale-locator fix in `rfc7296_auth_test.go` rides here |
| 2 | `2.15-3` | `parseAuthConfig`, one YANG leaf, its `peersEqual` entry, and the `ipsec-psk-hex` `.ci`. Self-contained, and the smallest of the build half |
| 3 | `3.6-1`, `4-4` | Certificate count and identity binding. Both touch the same PKI and remote-id surfaces, so splitting them costs a second audit of the same call sites. Carries the `ValidateIdentities` lift, the `remote_cert_chain_test.go` count change, and the first certificate-auth `.ci` |
| 4 | `3.6-2`, `3.6-3` | Hash-and-URL and the bounded fetcher. Its own commit and its own review, because it is the only one that adds an outbound network path |

Commit 4 is the natural boundary if OR-WP10-A splits the work.

**Each of commits 2, 3 and 4 adds a user-facing config leaf, so each owes a `.ci`**
(`ai/rules/testing.md`). `test/ipsec/` has no certificate or identity coverage
today, so this package is building that surface rather than extending it. Budget for it.

**All four are implementation, so they run on Opus 4.8, and each review runs on Opus 5
(`ai/rules/planning.md`). This design was produced in a planning phase; announce the
boundary before any code is written.**

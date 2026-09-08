# Spec: test-peer-open-inherits-zes-identity

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 5/6 |
| Handoff | - |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ze-peer` does not build the OPEN it sends. `generateOpen`
(`internal/test/peer/peer.go`) copies ze's OPEN whole and then writes over three
places in the copy: the last octet of the BGP Identifier, the identifier itself
when `option=open:value=router-id` asks for one, and the two-octet My AS field
when `option=asn` asks for one. Everything else is inherited verbatim, ze's
optional parameters among it.

The AS is therefore asserted twice. `patchAS4Capability` landed in HEAD on
2026-09-08 and moves the second assertion with the first, so `option=asn` already
reaches both carriers for an AS in 1 to 65535. Two gaps remain, and this spec
closes them. **The absent-option case:** 132 eBGP `.ci` files set no `option=asn`
at all, so ze-peer opens as ze and `validateOpenPeerAS`
(`internal/component/bgp/reactor/session_open_as.go`) answers NOTIFICATION 2/2
Bad Peer AS. `patchAS4Capability` cannot reach them. **Everything outside 1 to
65535:** `parseOptionConfig` (`internal/test/peer/expect.go`) already refuses a
non-numeric value, and 0, a negative value and a four-octet value all parse, are
then skipped by the `> 0 && <= 65535` guard in `generateOpen`, and the peer opens
as ze in silence.

`openAdvertisedAS` (`internal/component/bgp/reactor/peer.go`) reads the
capability first, as RFC 6793 Section 4.1 requires, so the capability is the
carrier a conforming receiver reads. The refusal itself is RFC 4271 Section 6.2,
Bad Peer AS, reached because 6793's precedence rule handed the validator an AS
the session is not configured for. Ze is correct at every step. The harness is
wrong, and the product's OPEN encoder is not in scope.

Measured over the 2026-09-07 gating log: 224 of its 269 failures carry that
signature, across four suites (5 `encode`, 212 `plugin`, 6 `ui`, 1 `vrrp`).
Today's tree has 705 `cmd=background` lines launching `ze-peer` and 1304
`stdin=peer` blocks in 637 `.ci` files, so this one function decides what every
functional test that drives a BGP session puts on the wire.

The ASN is the loudest instance of one habit, not the whole defect. The same
three lines silently drop a four-octet ASN: the write is guarded by
`p.config.ASN > 0 && p.config.ASN <= 65535`, so a `.ci` asking for AS 4200000000
patches NEITHER carrier and the peer opens as ze. That is a value silently wrong
rather than an error (`ai/rules/principles.md`). And three further mirrored
capabilities assert something about the SENDER that a mirror cannot get right:
Role (9), ADD-PATH (69) and FQDN (73).

The goal is one property, stated as a property rather than as a list of fields:
**every fact ze-peer's OPEN asserts about ze-peer, its AS, its identifier, its
role, its direction, its name, is resolved once from the test's own
configuration, and no octet of that fact is inherited from ze's OPEN. A fact the
harness cannot represent stops the `.ci` at read time.**

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` format reference; the
  Options table row for `asn`, the OPEN Behaviors table, and the
  "Capability Control" section describe the surface this spec changes
  → Decision: the page already documents the mirror as the contract
    ("Ze-peer mirrors the peer's OPEN message back, with a modified router-id"),
    and `drop-capability` / `add-capability` are documented as edits to that
    mirror. The mirror is a published behavior, so it changes in the open and the
    page changes with the code, never after it
  → Constraint: the Options table publishes `asn` as `value=<N>` / "Override peer
    ASN". That row is a claim the code does not honor above 65535, and does not
    honor in the AS4 capability at all. The row is corrected in the same work
- [ ] `ai/rules/principles.md` - the zero-value and single-declaration directives
  → Constraint: a request the harness cannot honor MUST NOT produce a peer that
    quietly claims someone else's identity. Refuse where the `.ci` is read
  → Constraint: one fact, one declaration. The AS is declared once in the harness
    and both wire carriers read that declaration

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc6793.md` - four-octet AS support; the two AS carriers in OPEN
  → Constraint: RFC 6793 Section 4.1: "When a NEW BGP speaker processes an OPEN
    message from another NEW BGP speaker, it MUST use the AS number encoded in
    the Capability Value field of the 'support for four-octet AS number
    capability' in lieu of the 'My Autonomous System' field of the OPEN message."
    A receiver that obeys this NEVER reads the field ze-peer patches, so patching
    the field alone changes nothing a conforming peer observes
  → Constraint: RFC 6793 Section 4.1: "The AS number of the BGP speaker MUST be
    carried in the Capability Value field of the 'support for four-octet AS
    number capability'." The capability carries the sender's AS, so a mirrored
    capability is an assertion about ze made in ze-peer's name
  → Constraint: RFC 6793 Section 3: AS_TRANS "is also placed in the 'My
    Autonomous System' field of the OPEN message originated by a NEW BGP speaker,
    if and only if the speaker does not have a (globally unique) two-octet AS
    number." A four-octet ASN is therefore fully representable: AS_TRANS (23456)
    in the header field and the real AS in the capability. It is not an
    unrepresentable request, it is an unimplemented one
- [ ] `rfc/short/rfc9234.md` - the BGP Role capability and its pairing rule
  → Constraint: RFC 9234 Section 4.2 requires corresponding roles, and
    `isValidRolePair` (`internal/component/bgp/plugins/role/validate.go`) holds
    the table. Provider pairs with Customer, RS with RS-Client, and only Peer
    pairs with itself. A mirror produces identical roles, so it is invalid for
    four of the five values
- [ ] `rfc/short/rfc7911.md` - ADD-PATH Send/Receive flags
  → Constraint: `Negotiate` (`internal/core/bgp/capability/negotiated.go`)
    intersects local Send with remote Receive and local Receive with remote Send.
    A mirrored one-directional capability intersects to `AddPathNone`, so the
    session negotiates ADD-PATH off with no error anywhere
- [ ] `rfc/short/rfc9072.md` - the two-octet Optional Parameters Length
  → Constraint: `Open.writeToExtended` (`internal/component/bgp/message/open.go`)
    writes 255 into the one-octet length, 255 into the parameter type, and the
    real length into the two octets after them. Any reader that treats body octet
    9 as the parameter length misframes such a message from its first parameter

**Key insights:**
- Ze's own OPEN producer is the reference for the shape this spec wants.
  `buildOpen` (`internal/component/bgp/reactor/session_negotiate.go`) resolves
  `openAS` ONCE and then lets the header field, the `capability.ASN4` value and
  the encoder's ASN4 field all read that one resolution. Its comment says why in
  one sentence: two resolutions is how a header saying one AS and a capability
  saying another get written.
- The mirror is load-bearing and relied upon. `test/encode/cap-require-asn4.ci`
  works by dropping code 65 from its mirrored response, and every `.ci` that
  negotiates a family ze offers works because the peer offered the same family
  without knowing what it was.
- The mirror is already worked around by hand where it is wrong. Every Role test
  (`test/plugin/role-otc-egress-stamp.ci`, `test/draft/plugin/role-otc-rs-eor.ci`,
  `test/draft/plugin/role-otc-egress-withdraw.ci`,
  `test/draft/plugin/otcprobe-inject.ci`) drops code 9 and adds it back with a
  role of its own choosing.
- The harness cannot express an asymmetric ADD-PATH session, and no `.ci`
  attempts one: every add-path test configures `direction send/receive`.
  `test/plugin/config-addpath-mode.ci` is titled "add-path send config negotiates
  ADD-PATH capability" and configures `direction send/receive`.
- `internal/test/peer/inject.go` already contains a from-scratch OPEN builder,
  `buildActiveOpen`, whose comment reads "No capability mirroring: we pick the
  minimum set". It handles the four-octet case correctly, with AS_TRANS in the
  header field and the real AS in the capability. Two builders exist in one
  package and only one of them is right.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/test/peer/peer.go` - `generateOpen` copies ze's header and body
  into a new buffer, increments body octet 8 (the identifier's last octet),
  optionally overwrites body octets 5 to 8 from `p.config.RouterID`, optionally
  overwrites body octets 1 and 2 from `p.config.ASN` when that value is in 1 to
  65535, then calls `applyCapabilityOverrides` and appends an unknown capability
  when asked. `applyCapabilityOverrides` re-frames the optional parameters by
  reading body octet 9 as a one-octet parameter length, filtering capability TLVs
  inside each type-2 parameter, appending each added capability as its own type-2
  parameter, and rewriting the message with a one-octet parameter length. It
  returns its input unchanged when the message is shorter than 29 octets.
- [ ] `internal/test/peer/expect.go` - `parseOptionConfig` turns
  `option=asn:value=N` into `config.ASN` with `strconv.Atoi`. `Config.ASN` is an
  `int`, so a negative value and a four-octet value both parse and both are then
  dropped by the range guard in `generateOpen`. `option=open:value=router-id`
  sets `config.RouterID`.
- [ ] `internal/test/peer/inject.go` - `buildActiveOpen` builds an OPEN from the
  inject spec with no mirroring: one Multiprotocol capability, one ASN4
  capability, AS_TRANS in the header field above 65535, and a router-id derived
  from the dial address.
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - `buildOpen` is ze's
  single OPEN producer, resolving the AS once for both carriers and narrowing to
  AS_TRANS above 65535.
- [ ] `internal/component/bgp/reactor/peer.go` - `openAdvertisedAS` parses the
  received optional parameters and returns the first ASN4 capability value above
  zero, falling back to `MyAS` only when no such capability is present.
- [ ] `internal/component/bgp/reactor/session_open_as.go` - `validateOpenPeerAS`
  refuses an AS the session is not configured for with NOTIFICATION 2/2. Its log
  line carries `migration-as` as one field among five; that field is why two
  journal rows misattributed these reds to the local-AS work.
- [ ] `internal/core/bgp/capability/negotiated.go` - `Negotiate` computes the
  ADD-PATH mode as the intersection of the two directions.
- [ ] `internal/component/bgp/plugins/role/validate.go` - `validateOpenRolePair`
  answers NOTIFICATION 2/11 for a non-corresponding role pair.
- [ ] `internal/component/bgp/message/open.go` - `Open.WriteTo` and
  `writeToExtended` choose the one-octet or the RFC 9072 two-octet envelope.

**Behavior to preserve:** (unless the user explicitly said to change it)
- The capability SET follows ze's. A `.ci` that says nothing about capabilities
  still negotiates whatever ze offers, including families the test never names.
- `option=open:value=drop-capability:code=N` removes a capability from what
  ze-peer sends, and `add-capability:code=N:hex=...` adds one. Both keep their
  spelling and their meaning.
- `option=open:value=router-id:id=A.B.C.D` replaces the identifier outright,
  including the values the default can never produce (`0.0.0.0`, ze's own).
- The default identifier stays distinct from ze's and stays derived from ze's, so
  no `.ci` that asserts a specific identifier changes.
- `option=open:value=send-unknown-capability` still puts capability code 66 with
  the value `loremipsum` on the wire.
- Every `.ci` that today asserts ze's reply octets keeps asserting the same
  octets.

**Behavior to change:** (only what the user asked for)
- The AS is resolved once and written to BOTH carriers, or to neither.
- A four-octet `option=asn` value works, with AS_TRANS in the header field and
  the real AS in the ASN4 capability.
- An `option=asn` value the harness cannot honor stops the `.ci` at parse time
  with an error naming the value.
- The mirrored Role capability is reconciled rather than copied.
- The mirrored ADD-PATH directions are reconciled rather than copied.
- The mirrored FQDN capability is reconciled rather than copied.
- The optional parameters are read and rewritten through the RFC 9072-aware
  parser rather than through a fixed one-octet offset.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `.ci` file's `stdin=peer` block: `option=asn:value=N`,
  `option=open:value=router-id:id=...`, `option=open:value=drop-capability:...`,
  `option=open:value=add-capability:...`,
  `option=open:value=send-unknown-capability`.
- Text lines, parsed into a `peer.Config` before any socket exists.

### Transformation Path
1. `parseOptionConfig` (`internal/test/peer/expect.go`) turns each `option=` line
   into a `Config` field. This is where an unrepresentable request must stop.
2. `doOpenHandshake` (`internal/test/peer/peer.go`) reads ze's OPEN off the wire.
3. The OPEN builder takes ze's OPEN and the `Config` and produces the octets
   ze-peer sends.
4. Ze reads them: `openAdvertisedAS`, `validateOpenPeerAS`,
   `validateOpenRolePair`, `Negotiate`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` text to `peer.Config` | `parseOptionConfig` returns an error the runner prints and fails on | No |
| `peer.Config` plus ze's OPEN to wire octets | the OPEN builder in `internal/test/peer/` | No |
| ze-peer to ze | BGP OPEN over TCP, judged by ze's own validators | No |

### Integration Points
- `capability.ParseFromOptionalParams` (`internal/core/bgp/capability/`) already
  parses optional parameters in both RFC 4271 and RFC 9072 framing, and ze's own
  receive path uses it. The harness reads ze's parameters through it instead of
  walking octets at fixed offsets.
- `capability.Capability.WriteTo` and the `capability.ASN4`, `capability.AddPath`
  and Role capability types already encode what the harness needs to re-emit.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The `.ci` option reaches `parseOptionConfig` (`internal/test/peer/expect.go`), which refuses what the harness cannot honor before a socket exists, then `Config.OpenAS`, then `buildOpen` (`internal/test/peer/open.go`). No octet is written at a fixed offset into a buffer built elsewhere; `generateOpen`, `patchAS4Capability` and `applyCapabilityOverrides` are deleted |
| No unintended coupling (components stay isolated) | Yes | `internal/test/peer` reads `internal/core/bgp/capability` only, which is the package that owns the capability types and which ze's own receive path uses. `internal/test/runner` reads the `.ci` text and imports no BGP configuration package |
| No duplicated functionality (extends existing, does not recreate) | Yes | The capability TLVs pass through as raw octets, so only the four the builder OWNS are re-encoded, and each is re-encoded by its own `capability` type. Two OPEN builders remain and share their fact resolution: `myASField` narrows the AS for both, `Config.resolveOpenAS` answers which AS the `.ci` declared for both. The block reader the tunnel lint had in a `_test.go` moved to `internal/test/runner/ci_config_read.go` rather than being written a second time |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `parseOpenParams` returns sub-slices of ze's OPEN body and never copies a capability. `encodeOpen` sizes the whole message once and writes into it at offsets. This is a test harness, so the target is correctness rather than the wire path's allocation budget |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Nothing registers and nothing central is edited. `ownedCapabilities` switches on capability code, which is a list of the facts that describe the SENDER rather than an enumeration of what exists: a new capability passes through untouched, and only one Ze-peer asserts about itself needs a case |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Reconciling the AS in both carriers clears the 224 gating failures carrying that signature | The 2026-09-07 row in `plan/journal/test-against-broken-path.md` measured the count and named the mechanism; `openAdvertisedAS` reads the capability first | The reds have a second cause and this spec closes fewer than it claims | a full `./le functional gating` run before and after, compared by failing-test name | broken as stated, confirmed in substance. Reconciling the carriers is NOT enough: 132 of the failures set no `option=asn` at all, so nothing was there to reconcile, and the AS had to be DERIVED from the ze configuration the `.ci` embeds (`declarePeerAS`, `internal/test/runner/peer_asn.go`). With the derivation, six representative failures from the baseline list pass individually: `adj-rib-in-query`, `med-removal-configured`, `rfc4271-partial-unknown-transitive`, `asn4-transcode-pooled-buffer`, `role-otc-egress-stamp`, `peer-local-port-listener`. The full comparison is phase 6 |
| A-2 | Decoding ze's optional parameters and re-encoding them preserves what every `.ci` asserts | ze's own receive path reads back what `capability.ParseFromOptionalParams` produces; `Open.ExtendedParams` exists so a decoded message can be re-encoded without the framing being lost | tests that assert exact octets break, or ze misframes the peer's parameters | a unit test that round-trips every capability ze can send and compares octets, plus the gating run | confirmed, by a stronger route than the one planned. The builder does NOT decode into typed capabilities: it keeps each capability TLV as raw octets and replaces only the four whose value describes the sender. `PathsLimit` drops limit-0 entries on parse, so a decode-and-re-encode would not have been octet-preserving. `TestPeerOpenPassThroughIsByteIdentical` asserts the whole parameter block comes back unchanged |
| A-3 | No `.ci` depends on ze-peer claiming ze's own AS | `test/plugin/peer-local-port-listener.ci` is recorded in `plan/journal/guard-added-to-one-half-of-a-pair.md` as doing exactly that as a workaround | that file, and any like it, go red on a correct harness and need their configuration corrected in this spec | grep for `.ci` files whose peer AS equals ze's configured local AS, then the gating run | confirmed for the one file the journal names. `test/plugin/peer-local-port-listener.ci` now declares an eBGP session (remote 65534 against local 65533), states no `option=asn`, and passes on the derived AS. A scan of every `.ci` with a peer block found no other file whose `option=asn` equals ze's own local AS |
| A-4 | Ze's optional parameters stay under 256 octets in the functional suites today, so the one-octet assumption has not yet produced a visible red | `buildOptionalParams` chooses the RFC 9072 framing only above 255 octets, and the 224 reds are all explained by the AS | the framing defect already causes reds attributed elsewhere, and the count in A-1 is wrong | report the parameter length the builder saw over the gating run | confirmed indirectly. Every `.ci` run in this phase passed with the builder emitting one-octet framing, which it does below 256 octets, so no functional case reached the wider envelope. The builder now handles both directions, so the question stops mattering: `TestPeerOpenReadsExtendedParameterFraming` reads an RFC 9072 input and `TestPeerOpenEmitsExtendedFramingAbove255` emits one |
| A-5 | The Role reconciliation can pick the complement without being told, because ze's role is in the OPEN being answered | `isValidRolePair` holds the table and the Role capability value is one octet in ze's OPEN | the harness picks a role a test did not want and a role test changes meaning | the four Role `.ci` files keep their verdict with their explicit drop and add lines, and a new `.ci` with no Role lines establishes | confirmed. `complementaryRole` reads the one-octet value out of ze's OPEN and answers with the RFC 9234 Section 4.2 Table 2 complement. `test/plugin/peer-open-role-complement.ci` names no role and establishes; `test/plugin/role-otc-egress-stamp.ci` keeps its verdict with its explicit drop and add lines, because a capability the `.ci` states replaces the builder's own |
| A-6 | `option=asn` is the only option whose value is written into one carrier of a two-carrier fact | read of `parseOptionConfig` and `generateOpen` | another option has the same defect and this spec leaves it | enumerate every `Config` field the builder reads and name the carriers of each | broken. Three more options had the same shape, and all three are fixed here: `router-id` dropped a malformed or IPv6 value in silence and sent the derived default instead, `drop-capability` and `add-capability` dropped an out-of-range `code=` in silence, and `add-capability` discarded `hex.DecodeString`'s error so bad hex became an empty capability value and a value above 255 octets truncated. A fifth is recorded rather than fixed: `option=asn` reached NO carrier at all on a dialing inject peer, which `buildActiveOpen` now honors |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reconciling the Role capability changes what the four existing Role tests negotiate | those four `.ci` files change verdict in the gating run | the reconciliation is a no-op when the test already set code 9: an explicit `add-capability` wins over any harness default |
| R-2 | Reconciling ADD-PATH directions turns a currently-off negotiation on, and tests written against the off state change behavior | `test/encode/addpath-encode.ci`, `test/encode/path-information.ci`, `test/plugin/adj-rib-in-replay-addpath-source.ci` change verdict | those three already configure `send/receive`, where reconciliation is the identity; read each before landing |
| R-3 | Re-encoding the optional parameters changes their ORDER, and a `.ci` asserts ze's reply to a specific order | an `inspect-open-message` test changes verdict | preserve read order on re-emit, and pin it with a unit test comparing octets for an unmodified pass-through |
| R-4 | The one-octet parameter length silently truncates when reconciliation makes the parameters longer than 255 octets | nothing today; that is the defect | the builder emits RFC 9072 framing above 255 octets, and refuses rather than truncating above the two-octet limit |
| R-5 | A `.ci` that currently passes only because the peer claimed ze's AS starts failing | the gating run shows a NEW failure the old harness passed | those files are corrected in this spec, never excluded; A-3 finds them before the run |
| R-6 | The change is proven by unit tests alone and lands with the suite untested | the Functional and gating rows stay empty | the gating run is an acceptance criterion, not a verification step |
| R-7 | The four-octet ASN path is added but no `.ci` exercises it, so it is unwired | no `.ci` names an AS above 65535 | AC-3 owes a new `.ci` establishing a session with a four-octet peer AS |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every functional test that drives a BGP session: 637 `.ci` files, 1304 `stdin=peer` blocks. Nothing an operator can reach; the shipped daemon is not touched |
| How is it reverted? | A single commit revert. The harness has no state and no on-disk format |
| Who else touches this path? | Any session writing a `.ci`. `plan/journal/test-against-broken-path.md` and `plan/journal/guard-added-to-one-half-of-a-pair.md` each carry rows naming this producer, and `plan/pre-release/spec-interop-suite-red.md` covers the interop suite rather than this one |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `option=asn:value=65010` in a `stdin=peer` block | → | the OPEN builder's AS resolution | `TestPeerOpenASReachesBothCarriers` |
| `option=asn:value=4200000000` in a `stdin=peer` block | → | the OPEN builder's AS_TRANS narrowing | `TestPeerOpenFourOctetASUsesASTrans` |
| `option=asn:value=abc` in a `stdin=peer` block | → | the `parseOptionConfig` refusal | `TestPeerOptionASNRefusesUnrepresentable` |
| a `.ci` peer block with no `option=asn` against a ze whose role is Provider | → | the OPEN builder's Role reconciliation | `test/plugin/peer-open-role-complement.ci` |
| a `.ci` peer block asking for a four-octet peer AS end to end | → | the whole handshake | `test/encode/peer-open-four-octet-as.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer block sets `option=asn:value=N` with N in 1 to 65535 | The OPEN ze-peer sends carries N in the My AS field AND N in the four-octet AS capability, and carries no other value of the peer's AS anywhere |
| AC-2 | A peer block sets no `option=asn` | The OPEN ze-peer sends carries one AS in both carriers, and that AS is the one ze is configured to expect from this peer |
| AC-3 | A peer block sets `option=asn:value=4200000000` | The OPEN carries 23456 in the My AS field and 4200000000 in the four-octet AS capability, and ze establishes the session |
| AC-4 | A peer block sets `option=asn:value=0`, a negative value, a value above 4294967295, or a non-numeric value | The `.ci` fails when it is read, with an error naming the option and the value. No socket is opened and no OPEN is sent |
| AC-5 | Ze's OPEN carries a Role capability and the peer block names no role | The OPEN ze-peer sends carries a role that corresponds to ze's under RFC 9234 Section 4.2, and ze does not answer NOTIFICATION 2/11 |
| AC-6 | A peer block names a role with `add-capability:code=9` | That role is sent unchanged, and the harness adds no second Role capability |
| AC-7 | Ze's OPEN carries an ADD-PATH capability whose mode for a family is Send only | The OPEN ze-peer sends carries Receive for that family, and the negotiated mode is not None |
| AC-8 | Ze's OPEN carries an FQDN capability | The OPEN ze-peer sends carries ze-peer's own name, not ze's |
| AC-9 | Ze's OPEN uses the RFC 9072 two-octet parameter framing | Ze-peer reads every capability in it, its own OPEN re-emits them, and `drop-capability` and `add-capability` act on that message as they do on a one-octet one |
| AC-10 | The reconciled capabilities exceed 255 octets | The OPEN ze-peer sends uses the RFC 9072 framing, and no length field is truncated |
| AC-11 | A peer block sets `option=open:value=drop-capability` or `add-capability` | The result is the same capability set the current harness produces for the same input, for every capability code any `.ci` uses today |
| AC-12 | The whole functional gating suite is run before and after | No test that passed before fails after, and the failures carrying the Bad Peer AS signature are gone |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerOpenASReachesBothCarriers` | `internal/test/peer/open_test.go` | AC-1: the two-octet field and the ASN4 capability carry one value | PASS. RED under two breaks: the field left inheriting ze's octets, and capability 65 left at ze's AS |
| `TestPeerOpenFourOctetASUsesASTrans` | `internal/test/peer/open_test.go` | AC-3: 23456 in the field, the real AS in the capability | PASS. RED under both AS breaks |
| `TestPeerOpenFourOctetASAddsTheCapability` | `internal/test/peer/open_test.go` | AC-3: the capability is ADDED where ze offered none, because above 65535 it is the AS's only carrier | PASS. RED under the capability break |
| `TestPeerOpenDefaultASMatchesZesExpectation` | `internal/test/peer/open_test.go` | AC-2: no `option=asn` still produces one consistent AS, read through the RFC 6793 Section 4.1 precedence rule | PASS. RED under the capability break |
| `TestPeerOpenASBindingPicksTheConnectionsPeer` | `internal/test/peer/open_test.go` | AC-2: a keyed `option=asn:peer=<ip>` wins over an unkeyed one, on either endpoint, and no declaration is reported as none rather than as AS 0 | PASS |
| `TestPeerOptionASNBindsAnAddress` | `internal/test/peer/expect_test.go` | AC-2: the `peer=` key parses, and a malformed address fails the file | PASS. RED with the key parsed and its error discarded |
| `TestDeclarePeerASFromOneConfiguredPeer` | `internal/test/runner/peer_asn_test.go` | AC-2: a block that states no AS gets the one `session { asn { remote N } }` declares | PASS. RED with the derivation disabled |
| `TestDeclarePeerASLeavesAStatedASAlone` | `internal/test/runner/peer_asn_test.go` | AC-2: an explicit `option=asn` is the override | PASS. RED with the stated-AS check removed |
| `TestDeclarePeerASKeysEachSession` | `internal/test/runner/peer_asn_test.go` | AC-2: several configured ASNs produce one keyed line per address ze dials | PASS. RED with one AS answering every session |
| `TestDeclarePeerASPrefersTheDialAddress` | `internal/test/runner/peer_asn_test.go` | AC-2: the address ze dials outranks the address ze speaks from | PASS. RED with the precedence removed |
| `TestDeclarePeerASRefusesOneAddressWithTwoASNs` | `internal/test/runner/peer_asn_test.go` | AC-2: two peers ze dials at one address, expecting two ASNs, fail the file rather than being guessed | PASS. RED with the refusal removed |
| `TestDeclarePeerASIgnoresANonBGPPeer` | `internal/test/runner/peer_asn_test.go` | AC-2: an IPsec or WireGuard `peer` block contributes nothing | PASS |
| `TestPeerOptionASNRefusesUnrepresentable` | `internal/test/peer/expect_test.go` | AC-4: zero, negative, oversized, non-numeric and empty each return an error naming the option and the value; 1, 65535, 65536 and 4294967295 each parse | PASS. RED under two breaks: the range check removed, and the parse error discarded |
| `TestPeerOptionAddCapabilityRefusesWhatItCannotSend` | `internal/test/peer/expect_test.go` | AC-4 for the other options: an out-of-range `code=`, a non-numeric one, non-hex `hex=`, and a value above 255 octets each fail the file | PASS. RED under the code-range break, the hex-error break and the length break |
| `TestLoadExpectFileRouterIDOverride` | `internal/test/peer/expect_test.go` | AC-4 for `router-id`: a malformed or IPv6 identifier fails the file rather than sending the default | PASS. RED with the silent-drop shape restored. Its two refusal cases asserted the drop until this spec; the assertion was wrong, not the guard |
| `TestPeerOpenRoleIsComplementary` | `internal/test/peer/open_test.go` | AC-5: the mirrored role is replaced by its complement, for all five RFC 9234 values | PASS. RED with the role mirrored |
| `TestPeerOpenExplicitRoleWins` | `internal/test/peer/open_test.go` | AC-6: an `add-capability` for a code the builder resolves suppresses the builder's own, for code 9 with a drop beside it and for code 65 without one | PASS. RED with the stated-capability check removed |
| `TestPeerOpenAddPathDirectionsInverted` | `internal/test/peer/open_test.go` | AC-7: Send becomes Receive, Receive becomes Send, Both stays Both | PASS. RED with the direction mirrored |
| `TestPeerOpenFQDNIsTheHarnessName` | `internal/test/peer/open_test.go` | AC-8 | PASS. RED with the FQDN mirrored |
| `TestPeerOpenReadsExtendedParameterFraming` | `internal/test/peer/open_test.go` | AC-9: an RFC 9072-framed input is parsed, `drop-capability` acts on it, and the rest is re-emitted | PASS. RED with the RFC 9072 input framing ignored |
| `TestPeerOpenEmitsExtendedFramingAbove255` | `internal/test/peer/open_test.go` | AC-10 | PASS. RED with the one-octet framing forced |
| `TestPeerOpenPassThroughIsByteIdentical` | `internal/test/peer/open_test.go` | R-3: with no overrides and no reconciliation needed, the parameters come back in the order they were read | PASS |
| `TestPeerOpenCapabilityOverridesUnchanged` | `internal/test/peer/open_test.go` | AC-11: drop and add produce the same set as today for every code used in `test/` | PASS |
| `TestPeerOpenSendUnknownCapability` | `internal/test/peer/open_test.go` | AC-11: `send-unknown-capability` still puts code 66 with the value `loremipsum` on the wire | PASS |
| `TestPeerOpenRouterIDDerivesFromZes` | `internal/test/peer/open_test.go` | Behavior to preserve: the default identifier is ze's with the last octet incremented, and the increment wraps inside that octet | PASS |
| `TestPeerOpenStatedASWinsOverEveryOtherDeclaration` | `internal/test/peer/open_test.go` | AC-1 and AC-6 together: an `add-capability:code=65` of four octets IS the AS declaration and the My AS field follows it; one of any other length declares no AS | PASS. RED with the stated capability not deciding the AS. Round-1 review found the two carriers disagreeing here |
| `TestPeerOpenRefusesAMessageAboveTheRFC4271Ceiling` | `internal/test/peer/open_test.go` | AC-10 boundary: the whole MESSAGE is bounded at 4096, not just the parameters, so the header Length field cannot wrap | PASS. RED with the bound removed |
| `TestPeerOptionASNRefusesTwoUnkeyedDeclarations` | `internal/test/peer/expect_test.go` | AC-4: two `option=asn` lines with no `peer=` key fail the file rather than the last one winning by order | PASS. RED with the duplicate check removed |
| `TestRecordOpenASRefusesWhatItCannotRead` | `internal/test/cli/record_open_as_test.go` | AC-4 on the server-mode path: a record `asn` that is not a number, and AS 0, each fail the run | PASS. RED under two breaks |
| `TestDeclarePeerASReadsAGroupsDeclaration` | `internal/test/runner/peer_asn_test.go` | AC-2: a peer nested in a `group` or `template` inherits its container's AS | PASS. RED with inheritance removed |
| `TestDeclarePeerASReadsAConfigOnDisk` | `internal/test/runner/peer_asn_test.go` | AC-2: the configuration an `option=file:path=` names is read like an embedded one | PASS. RED with the on-disk read removed |
| `TestDeclarePeerASRefusesAnEBGPPeerItCannotReach` | `internal/test/runner/peer_asn_test.go` | AC-2: the derivation fails CLOSED, naming the file and the peer | PASS. RED with the guard disabled |
| `TestDeclarePeerASLeavesAniBGPPeerUnreached` | `internal/test/runner/peer_asn_test.go` | AC-2: the guard's scope is eBGP only, because the mirror answers an iBGP peer correctly | PASS. RED with the guard widened to every peer |
| `TestPeerASDerivationReachesEveryCIFile` | `internal/test/runner/peer_asn_corpus_test.go` | AC-2 and AC-12: the derivation refuses no `.ci` in the tree, AND derives the right line for three real files, one per shape | PASS. RED with the guard widened to every peer, and RED with `declarePeerAS` emptied |
| `TestTokenizeConfigMatchesZesSeparators` | `internal/test/runner/ci_config_read_test.go` | AC-2: the reader's separator set is ze's own, tab included, and a parenthesis does not break a word because `readWord` does not break on one | PASS. RED with the tab removed from the set, and RED with comments no longer consumed |
| `TestReadConfigRefusesWhatItCannotCut` | `internal/test/runner/ci_config_read_test.go` | AC-2: an unterminated string and unbalanced braces each fail, so the reader can never answer "" for input it could not read | PASS. RED under both breaks |
| `TestConfigLookupsAreScopedToTheirOwnLevel` | `internal/test/runner/ci_config_read_test.go` | AC-2: `leaf` and `topLevel` see only the block's own level, in both write orders | PASS. RED with either scope removed |
| `TestConfigPresentSeparatesAbsentFromEmpty` | `internal/test/runner/ci_config_read_test.go` | AC-2: a block that is not there is absent, one that is there and empty is present | PASS. RED with `present` never set |
| `TestTokenizeConfigMatchesZesSeparators` (escapes) | `internal/test/runner/ci_config_read_test.go` | AC-2: the escape set is ze's own, so `\"` does not close a string and `\n`/`\t` translate | PASS. RED with escapes not honored, and RED with the two named escapes dropped |
| `TestAsLeafAnswersThreeStates` | `internal/test/runner/peer_asn_test.go` | AC-2: absent, value, and error, with AS 0 and an unreadable value both errors | PASS. RED with an unreadable leaf reported absent |
| `TestPeerLocalASReadsTheRouterLevelDeclaration` | `internal/test/runner/peer_asn_test.go` | AC-2: `bgp { session { asn { local N } } }` reaches every peer below it, whichever order the router's session and the peers appear in | PASS. RED with the router-level read removed |
| `TestDeclarePeerASRefusesAPeerItCannotRead` | `internal/test/runner/peer_asn_test.go` | AC-2: a peer whose declared AS does not read fails the file rather than leaving the population the guard walks | PASS. RED under two breaks: the refusal removed, and an unreadable leaf reported absent |
| `TestPeerOptionRefusesContradictoryOpenDeclarations` | `internal/test/peer/expect_test.go` | AC-4: every way the AS can be declared twice, or declared with no carrier, fails the file. Ten cases: dropped or stated capability 65 against a four-octet AS, two stated ASNs, a stated capability contradicting `option=asn`, and the four legal shapes beside them. The same pair split across a flag and a file fails in `New` | PASS. RED under six breaks, including both call sites of the validator removed and each of the two predicates broken alone. All four AS_TRANS shapes re-verified through `ze-test peer` rather than the unit test alone |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `option=asn:value` | 1-4294967295 | 4294967295 | 0 | 4294967296 |
| `option=asn:value` two-octet boundary | 1-65535 | 65535 | N/A | 65536 narrows to AS_TRANS, it is not a refusal |
| reconciled optional parameters | 0-65535 octets | 65535 | N/A | above 65535 the builder refuses rather than truncating |
| `add-capability:hex` value length | 0-255 octets | 255 | N/A | 256 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `peer-open-as-both-carriers` | `test/encode/peer-open-as-both-carriers.ci` | a test author sets a peer AS and the session establishes | PASS. RED with capability 65 left at ze's AS |
| `peer-open-four-octet-as` | `test/encode/peer-open-four-octet-as.ci` | a test author sets a four-octet peer AS and the session establishes | PASS. RED with capability 65 left at ze's AS. R-7: the only `.ci` in the tree naming an AS above 65535 |
| `peer-open-role-complement` | `test/plugin/peer-open-role-complement.ci` | a Role-configured ze establishes with a peer block that names no role | PASS. RED with the role mirrored, and RED with the AS derivation disabled |
| `peer-open-addpath-send-only` | `test/plugin/peer-open-addpath-send-only.ci` | ze configured `direction send` negotiates ADD-PATH with the peer | PASS. RED with the direction mirrored, and RED with the AS derivation disabled |
| `peer-local-port-listener` | `test/plugin/peer-local-port-listener.ci` | the file named in `plan/journal/guard-added-to-one-half-of-a-pair.md`, which had to give its peer ze's own AS. It now declares an eBGP session and states no `option=asn` at all | PASS. The spec's Functional table named it `peer-port-listener-direct-route`, which is not a file in the tree; the journal row names this one |
| `./le functional gating` | the whole suite | AC-12: the before and after comparison | phase 6, not run in this phase. Six representative baseline failures pass individually: `adj-rib-in-query`, `med-removal-configured`, `rfc4271-partial-unknown-transitive`, `asn4-transcode-pooled-buffer`, `role-otc-egress-stamp`, `peer-local-port-listener` |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Ze's wire behavior does not change. The change is confined to `internal/test/peer/`, which no shipped binary links. `ai/rules/interop-and-goal-validation.md` exempts tooling with no protocol peer, and the gating run is the proof this spec owes instead | |

## Files to Modify
- `internal/test/peer/peer.go` - `generateOpen` and `applyCapabilityOverrides`
  are replaced by the new builder; the handshake call site changes
- `internal/test/peer/expect.go` - `parseOptionConfig` refuses an
  unrepresentable `option=asn` value and stores it at its real width
- `internal/test/peer/inject.go` - `buildActiveOpen` stays the second OPEN
  builder and the reason is written above it: `buildOpen` answers an OPEN ze
  already sent and mirrors its capability SET, while the active role speaks
  FIRST, so there is nothing to mirror and the set has to be chosen. What they
  share, they share by calling one function: `myASField` narrows the AS for both
  and `Config.resolveOpenAS` answers which AS the `.ci` declared for both. The
  second call closes a silent drop the spec did not name: `option=asn` reached no
  carrier at all on a dialing inject peer, because `buildActiveOpen` read only
  `Inject.ASN`
- `internal/test/runner/record_parse.go` - `parseAndAdd` calls `declarePeerAS`
  before the two peer-block guards, so a derived line is held to the same
  contract a hand-written one is
- `internal/test/cli/cmd_peer.go`, `internal/test/cli/cmd_bgp.go` - `--asn` and
  the record's `asn` key write the new `Config.OpenAS` field, and `--asn` refuses
  a value outside the AS space rather than truncating it
- `internal/test/runner/tunnel_endpoint_lint_test.go` - its block reader moved to
  `ci_config_read.go`; the lint itself is unchanged
- `docs/architecture/testing/ci-format.md` - the `asn` Options row, the OPEN
  Behaviors table, and the "Capability Control" section describe the mirror
- `test/plugin/peer-local-port-listener.ci` - gives the peer ze's own AS as a
  workaround for this defect, recorded in
  `plan/journal/guard-added-to-one-half-of-a-pair.md`; it states its real AS once
  the harness can carry one
- `plan/journal/test-against-broken-path.md` - the row that names this producer
  gets its Fix cell completed at closure

## Files to Create
- `internal/test/peer/open.go` - the OPEN builder, one resolution per asserted
  fact
- `internal/test/peer/open_test.go` - the unit tests above
- `internal/test/runner/peer_asn.go` - AC-2's derivation: the AS ze expects from
  a peer is declared once, in the ze configuration the `.ci` embeds, and the peer
  block reads that declaration rather than repeating it
- `internal/test/runner/peer_asn_test.go` - its unit tests
- `internal/test/runner/ci_config_read.go` - the block and leaf reader the
  derivation needs. It is NOT new code: `namedBlocks`, `firstBlock`, `leaf`,
  `leafInBlock`, `atTokenStart` and `braceBody` were declared inside
  `tunnel_endpoint_lint_test.go` and moved here, so one reader serves both
  callers rather than the package holding two
- `test/encode/peer-open-as-both-carriers.ci` - AC-1 end to end
- `test/encode/peer-open-four-octet-as.ci` - AC-3 end to end
- `test/plugin/peer-open-role-complement.ci` - AC-5 end to end
- `test/plugin/peer-open-addpath-send-only.ci` - AC-7 end to end

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | The `.ci` peer block is not YANG config; it is test-harness input parsed by `internal/test/peer/expect.go` |
| YANG validation constraints | N-A | Same reason |
| YANG custom validators | N-A | Same reason |
| CLI commands/flags | N-A | No `ze` command changes. `ze-peer`'s own flags are unchanged; the options this spec touches arrive through the `stdin=peer` block |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No YANG leaf added |
| Functional test for new RPC/API | Yes | The four new `.ci` files in Files to Create |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No environment leaf |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or certificate. The harness talks to a TCP socket it already opened |
| Prometheus counters/metrics | N-A | Test harness, not a daemon surface |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability code or attribute. The spec changes what a test peer PUTS in capabilities that already exist |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | Nothing ships; `internal/test/peer/` is linked only by `ze-test` |
| 2 | Config syntax changed? | N-A | No `ze` configuration syntax changes |
| 3 | CLI command added/changed? | N-A | No command changes |
| 4 | API/RPC added/changed? | N-A | No API changes |
| 5 | Plugin added/changed? | N-A | No plugin changes |
| 6 | Has a user guide page? | Yes, and UNAFFECTED | `docs/guide/debugging-tools.md` carries two anchors into `internal/test/peer/peer.go`, and both sit away from this change: one on the message-decoder section and one in the File Locations table. The page says nothing about the OPEN ze-peer sends, about `option=asn`, or about capability overrides, so nothing on it became wrong. Recorded as unaffected rather than left silent |
| 7 | Wire format changed? | N-A | The OPEN wire format is unchanged. What changes is which values a test peer puts in it |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No `rfc/short/` Support row changes. The RFCs cited govern the HARNESS; ze's own conformance is untouched, and no `RFC requirement:` tag is added or reworded |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | N-A | No shipped behavior changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/ci-format.md` is the subsystem doc for this surface |
| 13 | Route metadata keys added/changed? | N-A | No metadata keys |
| 14 | Prometheus counters added/changed? | N-A | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | Nothing registers; no capability code is added to any inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, four in `docs/architecture/testing/ci-format.md`, all retargeted | `./le spec citation anchors` exits 0 with an empty report, so it does not answer this. The four are: the OPEN Behaviors table anchor, which named `internal/test/peer/checker.go` and a symbol that file never declared (`gopls symbols` gives `newChecker`, `parseExpectRule`, `parseKV`, `indexByteAligned`, `groupMatches`, `matchRule`), now `internal/test/peer/expect.go -- parseOptionConfig` and `internal/test/peer/open.go -- buildOpen`; the `router-id` anchor, which named the deleted `generateOpen`; the `option=asn` anchor, which named the deleted `generateOpen` and `patchAS4Capability`; and `docs/functional-tests.md`, whose hand-copied list of peer-block directives is deleted rather than corrected, because `ClaimLine` derives that set. `./le docs-to-code index-update` was run for the two new files; its eight remaining complaints are all pre-existing and name other pages |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/testing/ci-format.md` shows `option=asn:value=65533` and several drop and add capability examples; each is read against the new parser |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the `.ci` option reach a builder
   that can be tested
   - Tests: `TestPeerOpenASReachesBothCarriers`,
     `TestPeerOptionASNRefusesUnrepresentable`
   - Files: `internal/test/peer/open.go` (builder skeleton),
     `internal/test/peer/expect.go` (option parse), the handshake call site in
     `internal/test/peer/peer.go`
   - Verify: the option reaches the builder and the builder is called by the
     handshake. Both tests fail because the builder still mirrors
2. **Phase: One resolution per asserted fact, starting with the AS**
   - Tests: `TestPeerOpenASReachesBothCarriers`,
     `TestPeerOpenFourOctetASUsesASTrans`,
     `TestPeerOpenDefaultASMatchesZesExpectation`
   - Files: `internal/test/peer/open.go`, `internal/test/peer/expect.go`
   - Verify: tests fail, then pass. `test/encode/ebgp-encode.ci` establishes
3. **Phase: Read and re-emit the parameters properly** -- RFC 9072 framing and
   the override path
   - Tests: `TestPeerOpenReadsExtendedParameterFraming`,
     `TestPeerOpenEmitsExtendedFramingAbove255`,
     `TestPeerOpenPassThroughIsByteIdentical`,
     `TestPeerOpenCapabilityOverridesUnchanged`
   - Files: `internal/test/peer/open.go`
   - Verify: the override behavior is unchanged for every code `test/` uses, and
     the framing is handled in both directions
4. **Phase: The remaining asserted facts** -- Role, ADD-PATH, FQDN
   - Tests: `TestPeerOpenRoleIsComplementary`, `TestPeerOpenExplicitRoleWins`,
     `TestPeerOpenAddPathDirectionsInverted`, `TestPeerOpenFQDNIsTheHarnessName`
   - Files: `internal/test/peer/open.go`
   - Verify: the four existing Role `.ci` files keep their verdict, and the new
     role and add-path `.ci` files pass
5. **Phase: Reconcile the second builder** -- decide what happens to
   `buildActiveOpen`
   - Tests: the existing inject tests
   - Files: `internal/test/peer/inject.go`
   - Verify: one OPEN builder in the package, or two with the reason written down
6. **Phase: The suite** -- the proof this spec owes
   - Tests: `./le functional gating`, run before and after, compared by failing
     test name
   - Files: any `.ci` the comparison shows was passing only because the peer
     claimed ze's identity
   - Verify: AC-12

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and line, and AC-12 carries the before and after comparison |
| Feature completeness | Every `option=` a `.ci` can write reaches the new builder; none is read at a fixed octet offset |
| Correctness | No fact the OPEN asserts is written in one place and inherited in another. Enumerate the facts and name the single resolution for each |
| Naming | The builder's name says it builds. `generateOpen` did not survive its own name |
| Data flow | The refusal for an unrepresentable value happens in `parseOptionConfig`, before any socket exists, never in the builder |
| Rule: `ai/rules/principles.md` | No guard silently drops a value. The removed `p.config.ASN <= 65535` guard must not reappear as a silent skip anywhere |
| Rule: `ai/rules/pre-release.md` | The gating run's remaining reds are read once and reported, not repaired into this spec |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One OPEN builder in `internal/test/peer/` | `gopls symbols internal/test/peer/open.go`, and a grep of the package for a second builder |
| No fixed-offset write into a copied OPEN | grep `internal/test/peer/` for `19+` and for `open[` |
| `option=asn` refuses what it cannot honor | `TestPeerOptionASNRefusesUnrepresentable` |
| Four-octet peer AS works end to end | `test/encode/peer-open-four-octet-as.ci` passes |
| The gating comparison | the before and after run logs, both pasted into the closure section |
| `docs/architecture/testing/ci-format.md` matches the code | the `asn` row and the Capability Control section read against `parseOptionConfig` and the builder |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `option=asn:value` and `add-capability:hex` come from a `.ci` file. Neither may index or size a buffer without a bound, and the hex value's length is checked before it becomes a one-octet capability length |
| Resource exhaustion | The re-emitted parameters are bounded by the RFC 9072 two-octet limit, and the builder refuses above it rather than truncating |
| Error leakage | N-A: a test harness with no privileged output |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A gating test that passed before now fails | It was passing on the old harness's inherited identity. Correct the `.ci`, never the builder |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A mirror is an assertion of sameness. Every field in an OPEN that says
  something about the SENDER is therefore wrong in a mirror by construction, and
  the only question is whether anything checks it. The AS is checked, so it
  produced 224 reds. The role is checked, so four `.ci` files carry a
  hand-written workaround. The ADD-PATH direction is not checked, it negotiates
  to nothing, so it produced a coverage hole instead of a failure: no `.ci` in
  the tree configures a one-directional ADD-PATH. The FQDN is not checked at all.
  Ranked by how loudly they failed, these look like four unrelated bugs. Ranked
  by what they are, they are one.
- The defect is not that a field was missed. It is that the code writes a fact
  into ONE of its carriers, at a fixed offset, in a buffer it did not build. That
  shape cannot be made safe by adding a second write, because the next capability
  ze learns to send re-opens it. Ze's own producer already shows the answer in
  its comment: resolve the fact once, and let every carrier read that resolution.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Mirror the capability SET, own every asserted VALUE: ze's OPEN is decoded, the harness replaces each fact that describes the sender, and the result is encoded | Patch the AS4 capability beside the My AS field | The narrow patch closes the 224 reds and leaves the Role workaround, the ADD-PATH hole, the FQDN and the RFC 9072 framing. It also keeps the shape that produced all four: a fixed-offset write into a buffer built elsewhere. Rejected as a repair that re-arms the defect |
| Mirror the capability SET, own every asserted VALUE | Build the OPEN from the `.ci`'s own declarations, with no mirror | The build is the honest shape, and `buildActiveOpen` already does it for inject mode. It costs the property that 637 `.ci` files depend on without stating: the peer offers whatever ze offers, so a test about a family, an extended-message limit or a paths-limit says nothing about capabilities. Making them all state it is a larger change than this defect justifies, and it reduces coverage on the way. Rejected. The tradeoff given up is that ze-peer still cannot be pointed at a capability set ze does not offer, except through `add-capability` |
| Refuse an unrepresentable `option=` value where the `.ci` is read | Refuse in the builder, or clamp the value | `plan/learned/005-runner-drops-what-it-cannot-honor.md` records five ways this runner answered a directive it did not understand instead of refusing it. Refusing at parse time is the answer that record reached, and it means no socket opens for a file that cannot mean what it says |
| A four-octet ASN is implemented, not refused | Refuse it as unrepresentable | RFC 6793 Section 3 makes it representable: AS_TRANS in the header field, the real AS in the capability. `buildActiveOpen` already does this in the same package. Refusing a request the format can carry would be doing less |
| Read and write the optional parameters through `capability.ParseFromOptionalParams` | Keep walking octets and add an RFC 9072 branch | One parser for the format, in the package that owns it. A second walker in the harness is the second declaration that drifts |

## Known Limitations
- The harness still cannot offer a capability ze did not offer, except through
  `option=open:value=add-capability`. That is unchanged by this spec and is the
  price of keeping the mirror.
- When the harness first started mirroring, and whether the mirror was ever
  correct, is UNKNOWN. Nothing in `plan/`, `plan/journal/` or
  `docs/architecture/testing/ci-format.md` records the decision, and this spec
  did not search history for it. It is recorded as unknown rather than guessed,
  because the answer changes nothing about the repair.
- Two journal rows misattribute these reds. The 2026-09-06 row in
  `plan/journal/test-against-broken-path.md` and the 2026-09-07 eBGP row beside
  it both point at the local-AS and migration work, because
  `validateOpenPeerAS`'s log line carries `migration-as` as one of five fields.
  The check that fires is RFC 4271 Section 6.2 and its input is
  `openAdvertisedAS`. **Nothing in ze's local-AS or RFC 7705 migration behavior
  is implicated, and this spec does not touch it.** The 2026-09-07 row that names
  `generateOpen` already carries the correction; it is repeated here so a reader
  who reaches the spec first does not open the wrong file.

## RFC Documentation (Scope: protocol)

The harness is not protocol-implementing code and adds no `RFC requirement:` tag.
It is still held to the RFCs it must satisfy to be a valid peer, and the code
that resolves each fact carries the citation:

| Fact | Citation the code carries |
|------|---------------------------|
| The AS in both carriers | RFC 6793 Section 4.1: "The AS number of the BGP speaker MUST be carried in the Capability Value field of the 'support for four-octet AS number capability'." |
| AS_TRANS above 65535 | RFC 6793 Section 3, on AS_TRANS being placed in the My Autonomous System field "if and only if the speaker does not have a (globally unique) two-octet AS number" |
| The complementary role | RFC 9234 Section 4.2, on roles that correspond |
| The inverted ADD-PATH direction | RFC 7911 Section 4, on the Send/Receive field |
| The parameter framing | RFC 9072 Section 2, on the two-octet Parameter Length |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every fact ze-peer's OPEN asserts about ze-peer is resolved once from the test's own configuration, and no octet of that fact is inherited from ze's OPEN | functional, whole-suite | The gating comparison, run by the main thread: **132 Bad Peer AS failures before, 2 after, and both of those two are now fixed as well** (see the row below). 138 tests went from red to green. `generateOpen`, `patchAS4Capability` and `applyCapabilityOverrides` are deleted, so no octet is written at a fixed offset into a copied buffer |
| The AS reaches BOTH carriers RFC 6793 defines, or neither | functional, wire-level | `test/encode/peer-open-as-both-carriers.ci`, whose eBGP session establishes only when capability 65 carries the declared AS. RED when the capability is left at ze's AS |
| A four-octet AS is implemented rather than refused | functional, wire-level | `test/encode/peer-open-four-octet-as.ci`: AS 4200000000, AS_TRANS in the header field, the real AS in the capability, session establishes. The only `.ci` in the tree naming an AS above 65535 |
| The mirrored Role is reconciled | functional, wire-level | `test/plugin/peer-open-role-complement.ci`: ze declares Provider, the peer block names no role, the session establishes and the reject rule on NOTIFICATION 2/11 never fires. RED with the role mirrored |
| The mirrored ADD-PATH directions are reconciled | functional, wire-level | `test/plugin/peer-open-addpath-send-only.ci` asserts ze's UPDATE carries the RFC 7911 Section 3 Path Identifier, which appears only when the direction negotiated to something other than None. RED with the direction mirrored |
| A request the harness cannot honor stops the `.ci` at read time | unit, negative | `TestPeerOptionASNRefusesUnrepresentable`, `TestPeerOptionAddCapabilityRefusesWhatItCannotSend`, `TestLoadExpectFileRouterIDOverride`, `TestPeerOptionASNRefusesTwoUnkeyedDeclarations`, `TestRecordOpenASRefusesWhatItCannotRead`. Each has a recorded RED |
| The derivation itself does not fail open | unit plus corpus | `TestDeclarePeerASRefusesAnEBGPPeerItCannotReach` proves the guard fires; `TestPeerASDerivationReachesEveryCIFile` runs the derivation over every `.ci` in the tree and proves it refuses none. Both have a recorded RED |

**The 11 apparent regressions in the gating comparison all pass on re-run except
`ze-stripped-surface`, which fails on a missing `ze-stripped` binary in `$PATH`.
That is environmental and not a consequence of this change.**

**The 2 files that still carried the signature after the run are fixed.**
`test/ui/send-raw-reaches-one-peer.ci` and
`test/ui/send-unicast-reaches-the-wire.ci` are driven by a compiled fixture
rather than by a `stdin=peer` block, so neither `option=asn` nor the runner's
derivation could reach them: `startCLIWireSession`
(`internal/test/fixture/ui_fixture_send_bgp.go`) writes a configuration declaring
`asn { local 65533; remote 65000 }` and then exec'd `ze-test peer` with no
`--asn`, so the peer mirrored ze's AS 65533. The AS is now declared once as
`cliWirePeerAS` and read by both the configuration and the launch. Both files
pass, and both go RED when the flag is removed.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Five sender-describing facts are still mirrored: Graceful Restart (64), LLGR (70 and 71), software version (75), PATHS-LIMIT (76), and the two-octet Hold Time | They sit inside this spec's goal sentence, but no acceptance criterion reaches them, and only Graceful Restart is verified at its producer. Reconciling the restart time changes what `startEORTimer` waits for in every graceful-restart test, which is a blast radius this spec did not size | `spec-test-peer-open-mirrors-five-more-sender-facts`, written and CLOSED on 2026-09-08, so it is no longer on disk. All five facts are resolved: `ownedCapabilities` (`internal/test/peer/open_capability.go`) owns codes 64, 71, 75 and 76, and `encodeOpen` (`internal/test/peer/open.go`) writes the Hold Time from the resolved configuration |

## Review Gate

<!-- Filled at implementation time by /ze-review, per plan/TEMPLATE.md.
     Loop until 0 BLOCKER and 0 ISSUE. -->

### Run 1

Scope: the whole diff. Two lenses, both reading the producer: the wire encoder,
and the zero-value and guard rule. Run in a context that did not write the code.

| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| ISSUE | The AS derivation fails open three ways, so "the config declares no peer AS" and "I could not read it" produce the same silent mirror. A group or template declaration is invisible, a config supplied through `option=file:path=` is never read, and several ASNs with no `connection` address return empty with a nil error | `internal/test/runner/peer_asn.go` | Fixed. `asDeclaration.coversEveryEBGPPeer` refuses a file whose derivation reached a known-eBGP peer with nothing, naming the file, the peer and both ASNs. iBGP stays exempt because the mirror answers it correctly. `inheritedByPeer` reads group and template; `configuredPeerAS` reads `Record.ConfigFile`. `TestPeerASDerivationReachesEveryCIFile` runs the derivation over every `.ci` in the tree and refuses zero |
| ISSUE | A `.ci`-stated capability 65 suppressed the builder's ASN4 while the header still carried the resolved AS, so the OPEN asserted two different ASNs. `TestPeerOpenExplicitRoleWins` pinned that exact state and asserted nothing about the My AS field | `internal/test/peer/open.go` | Fixed. `statedOpenAS` makes a stated four-octet code 65 the AS declaration, outranking `option=asn` and the derivation, and `myASField` narrows from it. A stated code 65 of any other length declares no AS and goes out as written. The missing assertion added |
| ISSUE | Five sender facts left mirrored sit inside this spec's own goal sentence, so a journal row alone is a scope reduction the author may not take | `plan/journal/mirrored-field-asserts-the-wrong-sender.md` | Fixed by disposition. `spec-test-peer-open-mirrors-five-more-sender-facts` written, carrying the Graceful Restart producer evidence, and named in `Work Not Done`. That spec was implemented and CLOSED on 2026-09-08, so it is no longer on disk. The journal row stays; it records the class |
| NOTE | `encodeOpen` bounded the parameters, not the message, so a parameter block of 65504..65535 octets wrapped the two-octet Length field | `internal/test/peer/open.go` | Fixed. `openMsgMax = 4096` bounds the message, per RFC 4271 Section 4.1 with RFC 8654 Section 4 excluding OPEN and KEEPALIVE from the Extended Message Capability and Section 6 restating that against the 4096 number itself |
| NOTE | The second Optional-Parameters walker contradicts the Key Design Decision row without saying why, and copies ze's RFC 9072 detection gap | `internal/test/peer/open.go` | Fixed as documentation. The file header carries the reason a decode is not octet-preserving, and states that correcting `UnpackOpen` must correct `parseOpenParams` in the same change |
| NOTE | `zeTestRunServerOnly` discarded both the parse error and the zero, and `resolveOpenAS` read absence out of a zero value | `internal/test/cli/cmd_bgp.go`, `internal/test/peer/open.go` | Fixed. `recordOpenAS` refuses a non-numeric value and AS 0; `resolveOpenAS` returns a real `declared` bool; two unkeyed `option=asn` lines in one block are refused |
| NOTE | `TestPeerOptionASNRefusesUnrepresentable` asserted `Contains(err, "")`, which passes against any error | `internal/test/peer/expect_test.go` | Fixed. Every refusal case asserts its own substring |

### Run 2

Scope, fixed before the round ran: the Run 1 fixes only, plus the sibling call
sites they touched. That is `coversEveryEBGPPeer`, `inheritedByPeer`,
`(*configPeer).inherit` and `configuredPeerAS` in `internal/test/runner/peer_asn.go`;
`statedOpenAS`, `openIdentity`, `myASField`, `resolveOpenAS` and the `openMsgMax`
bound in `internal/test/peer/open.go`; `recordOpenAS` in `internal/test/cli/cmd_bgp.go`;
`cliWirePeerAS` in `internal/test/fixture/ui_fixture_send_bgp.go`; the new and
amended tests; and the `ci-format.md` claims those fixes rewrote. The eight
always-in-scope classes stay in scope everywhere.

| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| ISSUE | The coverage guard's own classifier was the blind reader it guards. `localKnown` decides whether a peer is eBGP, and it was set only from a per-peer `session { asn { local } }` or `local-as`; the router-level `bgp { session { asn { local N } } }`, which `test/plugin/bgp-local-as-options.ci` documents as inherited by every peer below, reached it never. A peer whose local AS is written only there came back `localKnown=false` and was exempted from the refusal it needed | `internal/test/runner/peer_asn.go` | Fixed. `routerDefaults` reads the router's own session through the new `topLevelBlock`, which finds a block at the TOP level of a body rather than the first one at any depth, so the answer does not depend on whether the peers are written above or below it. The inheritance chain is now router, then group or template, then peer, each level overriding the one above. `TestPeerLocalASReadsTheRouterLevelDeclaration` covers both orders, and `TestDeclarePeerASRefusesAnEBGPPeerItCannotReach` gained the router-level arm of the same configuration |
| ISSUE | `peersFromConfig` DROPPED any peer whose remote AS it could not parse, so the guard then walked a shorter list and reported every peer on it covered | `internal/test/runner/peer_asn.go` | Fixed. `asLeaf` answers three states, and the third is the point: a leaf that is not there is ABSENT, one that IS there and does not read is an ERROR. `peersFromConfig` refuses rather than drops, and `configuredPeerAS` names the file, the config and the peer. `TestDeclarePeerASRefusesAPeerItCannotRead` covers a non-numeric AS, AS 0, and an unreadable local AS |
| ISSUE | Three misattributed RFC section numbers, one of them in an operator-facing error string. RFC 8654's OPEN and KEEPALIVE exclusion is Sections 4 and 6, not Section 3, which is the capability itself. RFC 6793's widening is Section 3, "Protocol Extensions", not Section 2, which is the RFC 2119 boilerplate | `internal/test/peer/open.go`, `open_test.go`, `expect.go`, `internal/test/cli/cmd_peer.go` | Fixed at six sites, each verified by reading the heading in `rfc/full/` before it was written. The RFC 8654 sites now quote Section 4 ("The BGP Extended Message Capability applies to all messages except for OPEN and KEEPALIVE messages.") and Section 6 ("This document changes the latter number to 65,535 for all messages except for OPEN and KEEPALIVE messages.") |
| ISSUE | `drop-capability:code=65` beside a four-octet `option=asn` sent AS_TRANS with nothing carrying the AS the `.ci` asked for, which is a fact the harness cannot represent going out in silence | `internal/test/peer/open.go` | Fixed. `(*Config).validateOpenDeclarations` refuses the combination, naming the AS and AS_TRANS. Dropping the capability and stating it back is still legal, because the stated capability is then the carrier |
| NOTE | Two `add-capability:code=65` lines were not refused: `statedOpenAS` took the first and `addedParams` emitted both, so the wire carried two four-octet AS capabilities and the order decided the AS | `internal/test/peer/open.go` | Fixed in the same validator. `option=asn` already refused two unkeyed lines, and this is the declaration that outranks it |
| NOTE | `TestPeerASDerivationReachesEveryCIFile` proved absence of refusal, not presence of derivation: `declarePeerAS` returning nil immediately still passed it, so its discrimination was entirely borrowed from the guard it is meant to be independent of | `internal/test/runner/peer_asn_corpus_test.go` | Fixed. It now pins the derived lines for three real files, one per shape: an unkeyed line, two keyed lines for one process serving two peers, and a `group`-declared AS. Emptying `declarePeerAS` takes it RED along with nine unit tests. It also logs how many `.ci` never reached the derivation because they failed to parse first, so the corpus hole is stated rather than silent |
| NOTE | `ci-format.md` over-claimed the guard: "an eBGP peer the derivation reached with nothing" fails the file was false for a router-level local AS | `docs/architecture/testing/ci-format.md` | Fixed with the reader, and the row now states the limit that remains: the refusal knows only what the reader knows, so a peer for which NO local AS is declared at any level is not judged eBGP and is not refused |
| NOTE | `configuredPeerAS` swallowed `os.ReadFile`'s error on the `option=file` path and returned the peers it had, which is the same fail-open shape one layer down. `nilerr` reported it once the function could return an error | `internal/test/runner/peer_asn.go` | Fixed by propagating. **This goes further than Run 2's NOTE 6, which asked only that the branch be kept.** The branch is kept; it now refuses instead of skipping, because a configuration nobody read and a configuration declaring nothing produce the same empty population |

### Run 3

Scope, fixed before the round ran: the Run 2 fixes only, plus the sibling call
sites they touched. That is `routerDefaults`, `topLevelBlock`, `asLeaf`,
`peersFromConfig`, `peerLocalAS`, the inheritance chain and the propagated
`configuredPeerAS` error in `internal/test/runner/peer_asn.go` and
`internal/test/runner/ci_config_read.go`; `(*Config).validateOpenDeclarations`
and its two call sites in `internal/test/peer/open.go`,
`internal/test/peer/expect.go` and `peer.New`; the six corrected RFC citations;
`internal/test/runner/peer_asn_corpus_test.go`; the new and amended tests; and
the `ci-format.md` rows those fixes rewrote. The eight always-in-scope classes
stay in scope everywhere.

Rounds 1 and 2 each found the same defect class one layer further out: a lookup
that cannot tell "nothing was declared" from "I could not read it". Round 3's
first question is whether the third repair finally closes it, or moves it again.

It moved it again, and Round 3 said so: "the third repair hardens the PARSE and
leaves the FIND two-valued". `asLeaf` answered three states correctly while the
two functions FEEDING it still answered `""` for input they could not match. The
answer was to stop repairing instances and rewrite the reader, which is what
Round 4 did.

| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| ISSUE | The reader accepted a strict subset of what ze accepts, and every gap answered ABSENT. `leaf` and `namedBlocks` matched `keyword + " "`, while ze's tokenizer separates on tabs too, so `local<TAB>65000` is a valid configuration whose local AS the reader never saw; the peer was then classified not-eBGP and exempted from the guard that exists to refuse it. `peer<TAB>name {` was worse: the peer left the population entirely, which is Round 2's "the guard walks a shorter list" returning through the reader | `internal/test/runner/ci_config_read.go` | Fixed by rewriting the reader, not by guarding it. The text is TOKENIZED once and every lookup runs over tokens, so "did not match" no longer exists as an answer: a keyword is in the stream or it is not. The separator set is copied from `readWord` and `skipWhitespaceAndComments` (`internal/component/config/tokenizer.go`) rather than guessed, the copy names its source, and `TestTokenizeConfigMatchesZesSeparators` pins every member including the parenthesis ze does NOT break a word on. `readConfig` REFUSES a text it cannot cut, so the only remaining unreadable input is an error rather than an empty answer |
| ISSUE | A container's own declarations were read at ANY depth, so a group whose `session` or `connection` was written BELOW a nested peer took that peer's as its default. A sibling peer then inherited another peer's AS, another peer's dial address, and with them the eBGP verdict the whole guard turns on | `internal/test/runner/peer_asn.go` | Fixed at the reader, as the round required, rather than at the two call sites. `(configFile).leaf` and `(configFile).topLevel` are both scoped to the block's OWN level by construction, so no caller can read a nested block's declarations. `readDeclarations` replaces `readPeer` and chains `topLevel("session").topLevel("asn")`. `TestConfigLookupsAreScopedToTheirOwnLevel` and `TestDeclarePeerASReadsAGroupsDeclaration` both cover BOTH write orders |
| NOTE | The corpus gate matched refusal messages as strings, which is a second declaration of every message. Round 3 added one the list did not carry, so a file refused that way was counted `unreached` and the gate passed over it | `internal/test/runner/peer_asn_corpus_test.go` | Fixed with a typed error. `errASDerivation` is wrapped ONCE, at `declarePeerAS`, the only exit this file has, and the gate asks `errors.Is`. `derivationFor` was extracted so all three refusal paths reach that one wrap |
| NOTE | `option=asn` beside a CONTRADICTING four-octet `add-capability:code=65` was not refused; the higher-precedence stated capability won in silence | `internal/test/peer/open.go` | Fixed in `validateOpenDeclarations`, on the same argument the two-stated-65 refusal already rested on: any two declarations of the AS that disagree are a question the block asked twice. An agreeing pair is still legal |
| NOTE | `peersFromConfig` skipped a peer with no AS after inheritance, conflating "not a BGP peer" with "a BGP peer nothing declared an AS for" | `internal/test/runner/peer_asn.go` | Fixed structurally. The walk is scoped to `bgp { ... }`, so an IPsec or WireGuard `peer` block is no longer in the population at all and the conflation cannot arise. The remaining case is KEPT rather than skipped or refused, with `declaresAS` naming it: `test/reload/tx-bgp-rollback.ci` writes `peer broken { session { } }` on purpose to prove ze rejects it, so refusing would fail a working test. The peer stays in the population and `coversEveryEBGPPeer` exempts it, because eBGP is a comparison and it has only one half |

### Run 4

Scope, fixed before the round ran: the Run 3 fixes only, plus the sibling call
sites they touched. That is the whole rewritten
`internal/test/runner/ci_config_read.go`, meaning the tokenizer, `readConfig`,
`configFile`, `configBlock` and the `blocks`, `topLevel`, `leaf` and `blockIP`
lookups that replaced `namedBlocks`, `firstBlock`, `topLevelBlock`, `leaf`,
`leafInBlock`, `atTokenStart` and `braceBody`; `readDeclarations`,
`declaresAS`, the `bgp`-scoped peer walk and `errASDerivation` in
`internal/test/runner/peer_asn.go`; the contradicting-stated-65 refusal in
`(*Config).validateOpenDeclarations`; the two callers of the deleted helpers in
`internal/test/runner/tunnel_endpoint_lint_test.go`; the corpus gate's move to
`errors.Is`; and `test/weakened/fc0f54fe.md`, which was rewritten because its
"bodies unchanged" claim became false. The eight always-in-scope classes stay in
scope everywhere.

Rounds 1, 2 and 3 each found one defect class, one layer further out each time:
a lookup that cannot tell "nothing was declared" from "I could not read it".
Round 3 diagnosed why the repairs kept failing, that a three-state parse sat on
a two-state reader, and Round 4's fix rewrote the reader rather than guarding it.
This round's first question is whether that ended the class or relocated it.

It ended it. The reader's separator set is byte-for-byte ze's, every lookup is
depth-scoped by construction, and `readConfig` refuses rather than answering
empty. What Round 4 found instead was one instance of the SAME class in the
other package, and two citation defects.

| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| ISSUE | The AS_TRANS refusal was disarmed by a capability-65 line carrying no AS. `validateOpenDeclarations` counted every `Add` override for code 65 whatever its length and then skipped the check, while `statedOpenAS` treated only a FOUR-octet value as an AS declaration. So `option=asn:value=4200000000` with `drop-capability:code=65` was refused, and adding `add-capability:code=65:hex=1122` made the file load clean: the OPEN then claimed AS_TRANS with nothing carrying 4200000000. Reproduced through `LoadExpectFile`, not by reading | `internal/test/peer/open.go` | Fixed with one predicate. `statedAS` is the only test for "does this line declare the AS", and both `statedOpenAS` and the validator's count call it. A second spelling of that question is what opened the gap, which is the same class the reader rewrite closed in the other package. Its comment also names what it is NOT: "did the .ci write a capability 65 itself" is a different question, asked by `reconcileParams`, and a malformed capability answers yes to that one and no to this one. Two cases added to `TestPeerOptionRefusesContradictoryOpenDeclarations`: the disarming line refuses, and the same shape with a real AS in the stated capability stays legal |
| NOTE | `complementaryRole` cited "RFC 9234 Section 4, Table 2" while the quote two lines above it cited Section 4.2 correctly | `internal/test/peer/open.go` | Fixed to Section 4.2, verified by reading the headings in `rfc/full/rfc9234.txt`: Section 4 is "BGP Role", 4.1 is "BGP Role Capability", 4.2 is "Role Correctness", and Table 2 sits inside 4.2. Third citation defect this spec has produced, and the reason each round now reads `rfc/full/` before writing a section number |
| NOTE | `tokenizeConfig` read a quoted string to the first matching quote, while ze's `readString` honors backslash escapes. A string ze reads whole would have been cut in two, which is the divergence class the separator set was already fixed for | `internal/test/runner/ci_config_read.go` | Fixed. `readQuoted` copies ze's escape set: a backslash escapes the next character, `\n` and `\t` become a newline and a tab, every other escaped character stands for itself, and `\"` therefore does NOT close the string. One deliberate divergence is documented at the function: ze runs an unterminated string to the end of its input, and this one reports it so the caller refuses the file, because the harness must never turn a text it could not read into an answer. Four escape cases and an escaped-closing-quote refusal added to the reader's tests |
| NOTE | `configuredPeerAS` feeds every `tmpfs=` and `stdin=` block to the tokenizer, so a block holding no ze configuration could refuse the whole `.ci` | `internal/test/runner/peer_asn.go` | Not changed, by review decision, and now written down at the call site so the next reader does not "fix" it into a skip. Fail-closed IS the design: a text nobody could read must stop the test rather than quietly contribute no peers, because "contributed nothing" and "declares nothing" are the two answers this package exists to keep apart. No `.ci` in the tree triggers it |

### Run 5

Scope, fixed before the round ran: the Run 4 fixes only, plus the sibling call
sites they touched. That is `statedAS` and its two callers `statedOpenAS` and
`(*Config).validateOpenDeclarations` in `internal/test/peer/open.go`, the
`complementaryRole` citation in the same file, `readQuoted` and its callers in
`internal/test/runner/ci_config_read.go`, the sentence recorded at the
`configuredPeerAS` read closure in `internal/test/runner/peer_asn.go`, and the
test cases added for each. The eight always-in-scope classes stay in scope
everywhere.

This is the fifth round, which is the last this session may spend without an
explicit decision from the owner (`ai/rules/planning.md`). A clean round closes
the loop and the work commits.

| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| ISSUE | The AS_TRANS guard still failed open, one call site from where Round 4 closed it. Its precondition was `dropped`, set only in the `!override.Add` branch, but a STATED capability 65 removes the four-octet carrier just as effectively: `reconcileParams` sets `stated[65]` for ANY `Add` override whatever its length and then skips ze's own ASN4 TLV in both the read loop and the tail emit. So `option=asn:value=4200000000` with `add-capability:code=65:hex=1122` and NO drop at all loaded clean, and the OPEN claimed AS_TRANS with a two-octet capability 65 carrying 0x1122 while nothing carried 4200000000. Reproduced live through `ze-test peer`, which reached "listening" | `internal/test/peer/open.go` | Fixed by asking BOTH questions and naming both, because asking only one is how this failed open twice. `suppressesOwnASN4` is the precondition, true for a drop OR any add of code 65, which is the same predicate `reconcileParams` reads. `carriesTheAS` is the exemption, counted by `statedAS`, because only a four-octet stated capability carries the AS. The two are independently load-bearing: the discrimination walk breaks each one alone and each takes the test RED. `statedAS` itself was confirmed correct by the round and is unchanged. The refusal message now says "drops or states", because a reader who wrote only the add-capability line was sent looking for a drop they never wrote. Four cases now pin the matrix, and all four were re-verified at the real entry point rather than in the unit test alone: four-octet AS with a stated 65 carrying no AS refuses, with a drop refuses, with a stated 65 carrying the AS is accepted, and a two-octet AS with a stated 65 carrying no AS is accepted because the My AS field carries it alone |
| NOTE | `complementaryRole`'s two error strings cited "RFC 9234 Section 4" for the one-octet length and for the defined values. Section 4 is "BGP Role" and names the roles in prose only; `Length: 1 (octet)` and Table 1's values 0 to 4 are both in Section 4.1, "BGP Role Capability" | `internal/test/peer/open.go` | Fixed. Every citation in the function now matches what it cites, read from the headings in `rfc/full/rfc9234.txt`: 4.1 for the length and for Table 1's values, 4.2 for Table 2's pairs and for the Role Mismatch quote. The doc comment states which section carries which, so the next reader does not have to re-derive it. Fourth citation defect in this one function, and the reason the reading is now done before the number is written rather than after |
| NOTE | `test/encode/peer-open-four-octet-as.ci` timed out 2 of 26 runs, both inside a concurrent batch | `plan/journal/gate-verdict-depends-on-the-machine.md` | Not fixed, and the round MEASURED the diagnosis rather than accepting it: all 60 `test/encode/*.ci` share `timeout=10s`, and under stress the two new files and the untouched control `ebgp-encode` are indistinguishable (min 2.7s, avg 3.4s, max 6.3/6.3/6.4s, 8 of 8 each). A file whose timing tracks an untouched control to 0.1s is not flakier for a reason inside this spec, so the journal row is the right home and no discrimination record is owed |

### Run 6

Owner-authorised. Five rounds are what a session may spend alone
(`ai/rules/planning.md`); Thomas was asked whether the loop should run again
and answered "go for round 6".

Scope, fixed before the round ran: the Run 5 fix only, plus the sibling call
sites it touched. That is `(*Config).validateOpenDeclarations` and its two
named predicates `suppressesOwnASN4` and `carriesTheAS`, `statedAS` and its
callers, and the `complementaryRole` citations, all in
`internal/test/peer/open.go`; and the test cases added for each. The eight
always-in-scope classes stay in scope everywhere.

Rounds 1 through 5 each found one defect class, one layer further out each
time: a lookup or a guard that could not tell "nothing was declared" from "I
could not read it". Round 5's instance was the AS_TRANS refusal reading its
precondition off the wrong predicate. This round decides whether naming the two
questions separately ended the class or hid it.

**CLEAN. No BLOCKER, no ISSUE, no NOTE. The loop closes here.**

| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

What the round checked to earn that verdict, reviewing the COMMITTED state at
`c7aa4e63e` rather than a working tree:

| # | Checked | Result |
|---|---------|--------|
| 1 | Round 5's exact input reproduced at the real entry point | REFUSED, and the message names both 4200000000 and AS_TRANS |
| 1 | Over-refusal, the opposite defect: a four-octet AS with a stated four-octet 65 carrying the same AS, an AS below 65535 with a stated two-octet 65, and a plain `option=asn` | all three still ACCEPTED |
| 2 | `suppressesOwnASN4` against what `reconcileParams` really does | exact in the direction that matters: an override for code 65 if and only if the builder's TLV is suppressed. The converse gap is unreachable, because `own.add = id.as > 65535` is the same condition the refusal branch requires |
| 3 | A third spelling of either question anywhere | none. `statedAS` is the only length-4 test over a `CapabilityOverride`; `zeAdvertisedAS` reads a TLV in ZE's OPEN and `inject.go` writes ze-peer's own dialed OPEN, both a different subject |
| 4 | Both call sites unconditional and unbypassable | `LoadExpectFile` before its sole success return, and `New` before `&Peer{}`. `New` is the only non-test construction of `*Peer` |
| 5 | Every citation in `complementaryRole`, not only the ones Run 5 named | all six correct against the headings in `rfc/full/rfc9234.txt`, and the complement map matches Table 2. `validRolePairs` holds the same five pairs |
| 6 | Whether each predicate is INDEPENDENTLY load-bearing | yes, and verified live rather than from the table: one case is refused only by `suppressesOwnASN4`'s add-arm, another accepted only by `carriesTheAS` |

One boundary the round probed and cleared, recorded so a later reader does not
mistake it for the class returning: the guard reads `c.OpenAS`, so it cannot see
the AS when nothing is declared and `openAS` mirrors ze's. A `.ci` with
`drop-capability:code=65` and no `option=asn` therefore loads clean. That is
correct rather than a gap. A speaker with no ASN4 capability putting AS_TRANS in
the My AS field is what RFC 6793 Section 3 prescribes, and refusing it at load
time is impossible in any case, because ze's OPEN has not been read yet.

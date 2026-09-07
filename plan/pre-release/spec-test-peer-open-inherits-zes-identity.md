# Spec: test-peer-open-inherits-zes-identity

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ze-peer` does not build the OPEN it sends. `generateOpen`
(`internal/test/peer/peer.go`) copies ze's OPEN whole and then writes over three
places in the copy: the last octet of the BGP Identifier, the identifier itself
when `option=open:value=router-id` asks for one, and the two-octet My AS field
when `option=asn` asks for one. Everything else is inherited verbatim, ze's
optional parameters among it.

The AS is therefore asserted twice and the two assertions disagree. The patched
My AS field says what the `.ci` asked for; the inherited four-octet AS capability
still says ze's own AS. `openAdvertisedAS`
(`internal/component/bgp/reactor/peer.go`) reads the capability first, as
RFC 6793 Section 4.1 requires, and `validateOpenPeerAS`
(`internal/component/bgp/reactor/session_open_as.go`) answers NOTIFICATION 2/2
Bad Peer AS. Ze is correct at every step. The harness is wrong, and the product's
OPEN encoder is not in scope.

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
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Reconciling the AS in both carriers clears the 224 gating failures carrying that signature | The 2026-09-07 row in `plan/journal/test-against-broken-path.md` measured the count and named the mechanism; `openAdvertisedAS` reads the capability first | The reds have a second cause and this spec closes fewer than it claims | a full `./le functional gating` run before and after, compared by failing-test name | unvalidated |
| A-2 | Decoding ze's optional parameters and re-encoding them preserves what every `.ci` asserts | ze's own receive path reads back what `capability.ParseFromOptionalParams` produces; `Open.ExtendedParams` exists so a decoded message can be re-encoded without the framing being lost | tests that assert exact octets break, or ze misframes the peer's parameters | a unit test that round-trips every capability ze can send and compares octets, plus the gating run | unvalidated |
| A-3 | No `.ci` depends on ze-peer claiming ze's own AS | `test/plugin/peer-local-port-listener.ci` is recorded in `plan/journal/guard-added-to-one-half-of-a-pair.md` as doing exactly that as a workaround | that file, and any like it, go red on a correct harness and need their configuration corrected in this spec | grep for `.ci` files whose peer AS equals ze's configured local AS, then the gating run | unvalidated |
| A-4 | Ze's optional parameters stay under 256 octets in the functional suites today, so the one-octet assumption has not yet produced a visible red | `buildOptionalParams` chooses the RFC 9072 framing only above 255 octets, and the 224 reds are all explained by the AS | the framing defect already causes reds attributed elsewhere, and the count in A-1 is wrong | report the parameter length the builder saw over the gating run | unvalidated |
| A-5 | The Role reconciliation can pick the complement without being told, because ze's role is in the OPEN being answered | `isValidRolePair` holds the table and the Role capability value is one octet in ze's OPEN | the harness picks a role a test did not want and a role test changes meaning | the four Role `.ci` files keep their verdict with their explicit drop and add lines, and a new `.ci` with no Role lines establishes | unvalidated |
| A-6 | `option=asn` is the only option whose value is written into one carrier of a two-carrier fact | read of `parseOptionConfig` and `generateOpen` | another option has the same defect and this spec leaves it | enumerate every `Config` field the builder reads and name the carriers of each | unvalidated |

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
| `TestPeerOpenASReachesBothCarriers` | `internal/test/peer/open_test.go` | AC-1: the two-octet field and the ASN4 capability carry one value | |
| `TestPeerOpenFourOctetASUsesASTrans` | `internal/test/peer/open_test.go` | AC-3: 23456 in the field, the real AS in the capability | |
| `TestPeerOpenDefaultASMatchesZesExpectation` | `internal/test/peer/open_test.go` | AC-2: no `option=asn` still produces one consistent AS | |
| `TestPeerOptionASNRefusesUnrepresentable` | `internal/test/peer/expect_test.go` | AC-4: zero, negative, oversized and non-numeric each return an error naming the value | |
| `TestPeerOpenRoleIsComplementary` | `internal/test/peer/open_test.go` | AC-5: the mirrored role is replaced by its complement | |
| `TestPeerOpenExplicitRoleWins` | `internal/test/peer/open_test.go` | AC-6: an `add-capability:code=9` suppresses the harness default | |
| `TestPeerOpenAddPathDirectionsInverted` | `internal/test/peer/open_test.go` | AC-7: Send becomes Receive, Receive becomes Send, Both stays Both | |
| `TestPeerOpenFQDNIsTheHarnessName` | `internal/test/peer/open_test.go` | AC-8 | |
| `TestPeerOpenReadsExtendedParameterFraming` | `internal/test/peer/open_test.go` | AC-9: an RFC 9072-framed input is parsed and re-emitted | |
| `TestPeerOpenEmitsExtendedFramingAbove255` | `internal/test/peer/open_test.go` | AC-10 | |
| `TestPeerOpenPassThroughIsByteIdentical` | `internal/test/peer/open_test.go` | R-3: with no overrides and no reconciliation needed, the parameters come back in the order they were read | |
| `TestPeerOpenCapabilityOverridesUnchanged` | `internal/test/peer/open_test.go` | AC-11: drop and add produce the same set as today for every code used in `test/` | |

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
| `peer-open-as-both-carriers` | `test/encode/peer-open-as-both-carriers.ci` | a test author sets a peer AS and the session establishes | |
| `peer-open-four-octet-as` | `test/encode/peer-open-four-octet-as.ci` | a test author sets a four-octet peer AS and the session establishes | |
| `peer-open-role-complement` | `test/plugin/peer-open-role-complement.ci` | a Role-configured ze establishes with a peer block that names no role | |
| `peer-open-addpath-send-only` | `test/plugin/peer-open-addpath-send-only.ci` | ze configured `direction send` negotiates ADD-PATH with the peer | |
| `peer-port-listener-direct-route` | `test/plugin/peer-port-listener-direct-route.ci` | the existing red file, named in `plan/journal/guard-added-to-one-half-of-a-pair.md`, goes green with no edit to what it asserts | |
| `./le functional gating` | the whole suite | AC-12: the before and after comparison | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Ze's wire behavior does not change. The change is confined to `internal/test/peer/`, which no shipped binary links. `ai/rules/interop-and-goal-validation.md` exempts tooling with no protocol peer, and the gating run is the proof this spec owes instead | |

## Files to Modify
- `internal/test/peer/peer.go` - `generateOpen` and `applyCapabilityOverrides`
  are replaced by the new builder; the handshake call site changes
- `internal/test/peer/expect.go` - `parseOptionConfig` refuses an
  unrepresentable `option=asn` value and stores it at its real width
- `internal/test/peer/inject.go` - `buildActiveOpen` is the second OPEN builder
  in this package; it either becomes a caller of the new one or is recorded as
  deliberately separate, with the reason
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
| 6 | Has a user guide page? | Yes | `docs/guide/debugging-tools.md`, mapped to `internal/test/peer/peer.go` in `ai/CODE-TO-DOCS.md`: read what it says about ze-peer's OPEN and correct it, or record it as unaffected with the reason |
| 7 | Wire format changed? | N-A | The OPEN wire format is unchanged. What changes is which values a test peer puts in it |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No `rfc/short/` Support row changes. The RFCs cited govern the HARNESS; ze's own conformance is untouched, and no `RFC requirement:` tag is added or reworded |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | N-A | No shipped behavior changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/ci-format.md` is the subsystem doc for this surface |
| 13 | Route metadata keys added/changed? | N-A | No metadata keys |
| 14 | Prometheus counters added/changed? | N-A | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | Nothing registers; no capability code is added to any inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/pre-release/spec-test-peer-open-inherits-zes-identity.md` at implementation time and name every result. `docs/architecture/testing/ci-format.md` already carries a source anchor naming `internal/test/peer/checker.go` beside the OPEN Behaviors table |
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

## Review Gate

<!-- Filled at implementation time by /ze-review, per plan/TEMPLATE.md.
     Loop until 0 BLOCKER and 0 ISSUE. -->

### Run 1
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

### Run 2
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

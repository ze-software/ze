# Spec: radius-admin-eap

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | spec-radius-admin-chap (closed 2026-09-04) |
| Phase | 7/8 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `internal/component/radius/authenticator.go` -- `(*radiusAuthenticator).Authenticate`, single-shot today.
3. `internal/component/radius/packet.go` -- `verifyResponseMessageAuthenticator` and `VerifyCoAMessageAuthenticator`, the two verifiers; there is no signer.
4. `internal/component/ike/eap/peer.go` -- `NewPeerSession`, `(*PeerSession).Process`, `PeerResult`. This is the EAP peer this spec drives.
5. `internal/component/radius/config.go` -- `AuthMethod`, `authMethodNames` and
   `parseAuthMethod`, plus the `auth-method` leaf in
   `internal/component/radius/yang/ze-radius-conf.yang`. This is the selector
   this spec extends; spec-radius-admin-chap added it and closed 2026-09-04.

## Task

Ze's RADIUS admin backend sends one Access-Request and reads one answer. An
Access-Challenge is treated as an Access-Reject. That closes off every EAP
method, which is what most RADIUS deployments are actually configured for.

**The peer already exists.** `internal/component/ike/eap` implements the EAP
peer role with MD5-Challenge, MSCHAPv2 and TLS, driven through
`(*PeerSession).Process`, which takes an EAP Request and returns the peer's
Response. IKEv2 is its only caller today. Its non-test code imports
`internal/core/textbuf` and nothing else in the tree, so it is already a leaf
package sitting in the wrong tier.

**Ze answers EAP on the operator's behalf.** The operator types a password into
SSH or the web login. Ze holds that password, so it acts as the EAP peer itself
rather than relaying EAP frames to a client that does not speak EAP. Nothing in
the login transport changes, and the path works over SSH and web alike. An
earlier reading of this spec assumed SSH keyboard-interactive was a
prerequisite. It is not, and that reading is superseded.

What is missing is the RADIUS half:

1. RFC 3579 is not enrolled and `rfc/full/rfc3579.txt` is absent.
2. EAP-Message, attribute 79, is not in `internal/component/radius/dict.go`.
   `AttrMessageAuthenticator` at 80 is.
3. Nothing SIGNS an outbound Message-Authenticator. `packet.go` holds two
   verifiers and no signer.
4. `Authenticate` calls `(*Client).SendToServers` once and has no State loop.

**Design decisions (owner, 2026-09-03).** The EAP peer moves to
`internal/core/eap`, so RADIUS does not depend on IKE. The methods offered are
MD5-Challenge and MSCHAPv2, both password-driven and both already implemented.
EAP-TLS is out of scope: it needs an operator certificate and key, which is a
different credential model and a different config surface.

## Required Reading

### Architecture Docs
- [x] `docs/guide/radius.md` -- the shipped admin backend, its chain semantics and
  its Access-Challenge branch.
  → Constraint: the Access-Challenge branch stops being a rejection for the EAP
  methods and stays a rejection for PAP and CHAP.
- [x] `ai/rules/architecture.md` -- tier is dependency direction.
  → Decision: the EAP peer belongs in `internal/core/`, which is where a leaf
  with no component dependency belongs. `./le tier check` is the gate.
- [x] `docs/research/l2tpv2-ze-integration.md` -- named as the design document by
  `internal/component/radius/config.go`, a file this spec edits.
  → Constraint: it describes the L2TP SUBSCRIBER path and says nothing about
  admin login, so it constrains this spec only by keeping the two paths separate.

### RFC Summaries (Scope: protocol)
- [x] RFC 3579 Section 3.1 -- EAP-Message, attribute 79. "If multiple
  EAP-Message attributes are contained within an Access-Request or
  Access-Challenge packet, they MUST be in order and they MUST be consecutive
  attributes", and "Multiple EAP packets MUST NOT be encoded within EAP-Message
  attributes contained within a single Access-Challenge, Access-Accept,
  Access-Reject or Access-Request packet."
  → Constraint: one EAP packet per RADIUS packet, split at 253 octets into
  consecutive attributes, reassembled by concatenation on the way in.
- [x] RFC 3579 Section 3.1 -- "Therefore the Message-Authenticator attribute
  MUST be used to protect all Access-Request, Access-Challenge, Access-Accept,
  and Access-Reject packets containing an EAP-Message attribute", and "A NAS
  supporting the EAP-Message attribute MUST calculate the correct value of the
  Message-Authenticator and MUST silently discard the packet if it does not
  match the value sent."
  → Constraint: ze signs every EAP Access-Request and discards any reply whose
  Message-Authenticator does not verify. Discard, not reject: a discarded reply
  leaves the request outstanding for retransmission.
- [x] RFC 3579 Section 3.2 -- "Message-Authenticator = HMAC-MD5 (Type,
  Identifier, Length, Request Authenticator, Attributes)", and "When the message
  integrity check is calculated the signature string should be considered to be
  sixteen octets of zero."
  → Constraint: the signer writes 16 zero octets, computes over the whole
  packet, then overwrites them.
- [x] RFC 2865 Section 5.24 -- State. "This Attribute is available to be sent by
  the server to the client in an Access-Challenge and MUST be sent unmodified
  from the client to the server in the new Access-Request reply to that
  challenge", and "the client MUST NOT interpret the attribute locally. A packet
  must have only zero or one State Attribute."
  → Constraint: State is copied byte for byte and never parsed, and a reply
  carrying two State attributes is malformed.
- [x] RFC 3748 -- already enrolled, with the peer role gated. `rfc/short/rfc3748.md`
  carries the packet format, the Identifier and Type rules, one method per
  conversation, and four silent discards.
  → Constraint: the peer's obligations are met by the existing package. This
  spec adds no EAP-layer behavior and must not weaken what that ledger proves.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/radius/authenticator.go` -- `(*radiusAuthenticator).Authenticate`
  builds one packet, calls `(*Client).SendToServers` once, and switches on the
  reply code. `CodeAccessChallenge` falls to the rejection branch.
- [x] `internal/component/radius/packet.go` -- `VerifyCoAMessageAuthenticator`
  and `verifyResponseMessageAuthenticator` both VERIFY. Neither signs, and
  neither is reusable as a signer: they compare with `hmac.Equal`.
- [x] `internal/component/radius/dict.go` -- `AttrState`, `AttrMessageAuthenticator`
  and `CodeAccessChallenge` exist. Attribute 79 does not, and no `EAP` symbol
  names a RADIUS attribute; the only EAP spelling is `ErrorCauseInvalidEAPPacket`.
- [x] `internal/component/ike/eap/peer.go` -- `NewPeerSession(method uint8,
  identity, password string)` builds a peer for ONE method, which it NAKs
  toward. `(*PeerSession).Process` takes a `*Packet` and returns a `PeerResult`.
  `maxEAPRounds` already bounds the conversation and `ErrTooManyRounds` reports
  the bound.
- [x] `internal/component/ike/eap/eap.go` -- `DecodePacket` and `(*Packet).Encode`
  are the EAP wire boundary this spec encapsulates.
- [x] `internal/component/ike/eap/` imports -- non-test code names
  `internal/core/textbuf` only. `internal/component/ike/wire` appears in test
  files alone (`rfc7296_eap_test.go`, `eap_mschapv2_empty_response_test.go`).

**Behavior to preserve:** PAP and CHAP unchanged, including their treatment of
an Access-Challenge as a rejection; profile mapping; the fall-through on an
unreachable server; every RFC 3748 obligation the EAP peer already proves; the
IKEv2 caller of that peer.

**Behavior to change:** a bounded Access-Challenge loop for the two EAP methods,
and the attribute plumbing RFC 3579 requires around it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
An operator SSH or web login, exactly as for PAP and CHAP. The plaintext
password reaches `(*radiusAuthenticator).Authenticate` in `authz.AuthRequest`.

### Transformation Path
1. `ExtractConfig` reads `auth-method` as `eap-md5` or `eap-mschapv2`.
2. `Authenticate` builds an `eap.PeerSession` for that method from the username
   and password.
3. The first Access-Request carries the peer's EAP-Response/Identity in
   EAP-Message attributes, plus a Message-Authenticator computed over the whole
   packet with the signature field zeroed.
4. On Access-Challenge: verify the reply's Message-Authenticator, discard on
   mismatch, concatenate its EAP-Message attributes into one EAP packet, feed it
   to `(*PeerSession).Process`, and send the response in a new Access-Request
   carrying the State attribute unmodified.
5. On Access-Accept or Access-Reject: verify the Message-Authenticator, then
   hand the reply to the existing profile-mapping and rejection branches, which
   do not change.
6. The loop is bounded by the peer's own `maxEAPRounds` and by the
   authenticator's existing time budget.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → method selection | `auth-method` enum gains two values | [ ] |
| Authenticator → EAP peer | `eap.NewPeerSession` and `(*PeerSession).Process` | [ ] |
| EAP packet → RADIUS attributes | split at 253 octets into consecutive EAP-Message attributes | [ ] |
| RADIUS attributes → EAP packet | concatenate consecutive EAP-Message values | [ ] |
| Authenticator → server, per round | Message-Authenticator signed over the whole packet | [ ] |

### Integration Points
- `internal/core/eap/` -- the moved EAP peer package.
- `internal/component/ike/` -- its import path updated by the move.
- `internal/component/radius/dict.go` -- `AttrEAPMessage`.
- `internal/component/radius/packet.go` -- the Message-Authenticator signer.
- `internal/component/radius/authenticator.go` -- the challenge loop.
- `internal/component/radius/yang/ze-radius-conf.yang` -- two enum values.

### Architectural Verification
The move puts a package with no component dependency in the tier that has none,
which `./le tier check` enforces. Nothing gains a central switch: the method
enum is the one switch, with one arm per credential, and the EAP arms select a
peer method rather than adding a code path per method.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The EAP peer's non-test code has no component dependency, so the move is mechanical | its imports name `internal/core/textbuf` only | The move needs a larger untangle | read at the producer | confirmed |
| A-2 | `(*PeerSession).Process` is transport-neutral and needs only the EAP packet | its parameter is `*Packet`, decoded by `DecodePacket` | The peer would need a carrier abstraction | read at the producer | confirmed |
| A-3 | Ze holds the password when `Authenticate` runs | `request.Password` reaches the attribute list today | The whole spec is N/A | read at the producer | confirmed |
| A-4 | The two `ike/wire` test references can be re-homed without losing what they assert | both are test files naming `wire.PayloadEAP` | The IKE-carrier assertions must stay in IKE, splitting the test files | AC-12 | unvalidated |
| A-5 | `maxEAPRounds` bounds the RADIUS conversation adequately | the peer counts every `Process` call | An unbounded server could stall the login inside the time budget | AC-9 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The package move collides with another session working in IKE | a conflicting edit under `internal/component/ike/` | Do the move as its own commit, first, and land it before the RADIUS work |
| R-2 | The RFC 3748 ledger's tagged tests break on the import path change | `./le rfc check` names rfc3748 rows | The move changes no behavior, so a red there is a path fix, not a conformance fix |
| R-3 | A server that never concludes holds the login open | a login that neither accepts nor rejects | The peer's round cap and the authenticator's time budget both bound it |
| R-4 | The signer and the verifier disagree about which octets are covered | Message-Authenticator mismatches against a real server | The interop scenario against FreeRADIUS is the proof; a mock that shares ze's code would not be |

## Blast Radius

Three areas. `internal/core/eap` is new and is the old package moved. Every
importer under `internal/component/ike/` changes its import path and nothing
else. `internal/component/radius` gains an attribute, a signer and a loop. The
IKEv2 behavior is unchanged, and the RFC 3748 ledger must stay green through the
move.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `auth-method eap-mschapv2` in the config tree | → | the first Access-Request carries EAP-Message and Message-Authenticator | `TestRadiusAdminEapReachesTheWire` |

The wiring test drives the config tree, so a leaf that parses and never reaches
the authenticator fails it.

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `auth-method` absent, `pap`, or `chap` | No EAP-Message and no EAP behavior; the Access-Challenge branch still rejects |
| AC-2 | `auth-method eap-md5` or `eap-mschapv2` | The first Access-Request carries the peer's EAP-Response/Identity in EAP-Message attributes |
| AC-3 | Any Access-Request carrying EAP-Message | It carries a Message-Authenticator, computed per RFC 3579 Section 3.2 with the signature field zeroed during the HMAC |
| AC-4 | An EAP packet longer than 253 octets | It is split into consecutive EAP-Message attributes, in order, with no other attribute between them |
| AC-5 | A reply carrying several EAP-Message attributes | Their values are concatenated into one EAP packet before decoding |
| AC-6 | An Access-Challenge whose Message-Authenticator does not verify | The reply is silently discarded, per RFC 3579 Section 3.1. It is not treated as a rejection |
| AC-7 | An Access-Challenge carrying State | The next Access-Request carries that State byte for byte, and ze never parses it |
| AC-8 | An Access-Challenge with no State | The next Access-Request carries no State attribute |
| AC-9 | A server that challenges without concluding | The login ends with an error once the peer's round cap or the time budget is reached, and the chain falls through to the next backend |
| AC-10 | A completed EAP exchange ending in Access-Accept | Profiles map exactly as the PAP path maps them, and the session is tagged `source=radius` |
| AC-11 | An Access-Reject at any round | The chain stops, as it does for PAP |
| AC-12 | After the package move | `./le tier check` passes, every IKE test still passes, and the RFC 3748 ledger is unchanged |
| AC-13 | A single RADIUS packet | It carries at most one EAP packet, per RFC 3579 Section 3.1 |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `auth-method eap-mschapv2` and logs in over SSH | login → EAP identity → challenge → response → Access-Accept → profiles | `test/plugin/aaa-radius-eap.ci` |
| 2 | Sets `auth-method eap-md5` and logs in over the web UI | same, over the web login form | `test/plugin/aaa-radius-eap.ci` |
| 3 | Changes nothing and logs in | PAP, unchanged | `test/plugin/aaa-radius-admin.ci`, still green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEAPMessageSplitsAtTheAttributeLimit` | `internal/component/radius/eap_test.go` | AC-4 | |
| `TestEAPMessageConcatenatesOnTheWayIn` | `internal/component/radius/eap_test.go` | AC-5 | |
| `TestSignMessageAuthenticatorMatchesRFC3579` | `internal/component/radius/packet_test.go` | AC-3 | |
| `TestSignAndVerifyMessageAuthenticatorAgree` | `internal/component/radius/packet_test.go` | AC-3 | |
| `TestRadiusAdminEapChallengeLoopCarriesState` | `internal/component/radius/authenticator_test.go` | AC-7, AC-8 | |
| `TestRadiusAdminEapDiscardsUnauthenticatedChallenge` | `internal/component/radius/authenticator_test.go` | AC-6 | |
| `TestRadiusAdminEapStopsAtTheRoundCap` | `internal/component/radius/authenticator_test.go` | AC-9 | |
| `TestRadiusAdminEapOneEAPPacketPerRadiusPacket` | `internal/component/radius/authenticator_test.go` | AC-13 | |
| `TestRadiusAdminEapReachesTheWire` | `internal/component/radius/aaa_test.go` | Wiring, AC-2 | |
| `TestExtractConfigAuthMethodEap` | `internal/component/radius/config_test.go` | AC-1 | |

### Boundary Tests (numeric inputs)
| Input | Boundary | Expected |
|-------|----------|----------|
| EAP packet of 253 octets | one attribute exactly | one EAP-Message, Length 255 |
| EAP packet of 254 octets | one octet over | two consecutive EAP-Message attributes, 253 and 1 |
| EAP packet of 0 octets | empty | refused before the packet is built; an empty EAP-Message is not sent |
| Message-Authenticator | Length 18 | 16 octets of HMAC after the two header octets |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `aaa-radius-eap` | `test/plugin/aaa-radius-eap.ci` | EAP admin login accepted over a full challenge exchange, log shows `source=radius` | done |

### Interop Tests (Scope: protocol)
| Scenario | Peer implementation | Asserts |
|----------|--------------------|---------|
| `radius-admin-eap-freeradius` | FreeRADIUS configured for EAP-MSCHAPv2 | A real server accepts ze's Message-Authenticator, its EAP-Message framing and its State handling. A mock built from ze's own encoder would prove none of these |

The scenario is registered in `scenarioCheckerMap`
(`internal/le/interoplab/radius/radius.go`) and its checker is `checkEAP`
(`checkers.go`). Its polarities run with no container in
`TestEAPCheckerPolarities`.

## Files to Modify
- `internal/component/ike/**` -- import paths only, from the move.
- `internal/component/radius/dict.go` -- `AttrEAPMessage`.
- `internal/component/radius/packet.go` -- the Message-Authenticator signer.
- `internal/component/radius/authenticator.go` -- the challenge loop.
- `internal/component/radius/config.go` -- two more `auth-method` values.
- `internal/component/radius/yang/ze-radius-conf.yang` -- two more enum values.
- `docs/guide/radius.md` -- the methods, the Access-Challenge behavior, the chain.
- `rfc/enrolled.txt` -- rfc3579.

## Files to Create
- `internal/core/eap/` -- the moved package.
- `internal/component/radius/eap.go` -- EAP-Message split and concatenation.
- `internal/component/radius/eap_test.go`.
- `rfc/full/rfc3579.txt`, `rfc/short/rfc3579.md`.
- `test/plugin/aaa-radius-eap.ci`.
- `test/interop/scenarios/radius-admin-eap-freeradius/`.

### Integration Checklist
- [ ] `./le tier check` passes after the move.
- [ ] Both new enum values complete in the CLI schema.
- [ ] The signer has a non-test caller.
- [ ] `./le repository generate` re-run for the moved package.

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/guide/radius.md` -- the two methods, and the Access-Challenge
      paragraph, which currently states an unconditional rejection.
- [ ] `docs/features.md` -- the RADIUS row.
- [ ] `docs/features/rfc-status.md` -- regenerated from `rfc/short/`.
- [ ] `ai/CODE-TO-DOCS.md` / `ai/DOCS-TO-CODE.md` -- regenerated for the move.
- [ ] Any page naming `internal/component/ike/eap` by path.

## Implementation Steps

### Implementation Phases
1. **Phase: The move, alone.** `internal/component/ike/eap` to
   `internal/core/eap`, import paths updated, the two test references to
   `ike/wire` re-homed. No behavior change. Its own commit, landed before
   anything else, because it touches a component another session works in.
2. **Phase: RFC 3579 enrolment.** Fetch the text, run the enrolment walk, produce
   `rfc/short/rfc3579.md` with requirement ids for the obligations quoted above.
3. **Phase: The signer.** Message-Authenticator signing, proven against a vector
   the producer did not compute, and proven to agree with the existing verifier.
4. **Phase: The attribute.** EAP-Message split and concatenation, with the
   boundary cases above.
5. **Phase: Wiring first, then the loop.** The config-driven wiring test RED
   before the loop exists, then the challenge loop.
6. **Phase: Failure paths.** Unverified reply discarded, round cap, no State.
7. **Phase: Functional and interop.**
8. **Phase: Docs.**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| The move changed nothing | The IKE tests and the RFC 3748 ledger are as green as they were before it |
| Signer coverage | The HMAC is over Type, Identifier, Length, Request Authenticator and every attribute, with the signature octets zeroed |
| Discard, not reject | An unverified reply leaves the request outstanding; it does not stop the chain |
| State opacity | State is copied, never parsed, and never logged |
| One EAP packet per RADIUS packet | Asserted, not assumed |
| PAP and CHAP untouched | Their attribute lists and their Access-Challenge handling are unchanged |

### Deliverables Checklist
| Deliverable | Verification method | Status |
|-------------|--------------------|--------|
| EAP peer in `internal/core/eap` | `./le tier check` | |
| RFC 3579 enrolled | `rfc/short/rfc3579.md` exists and `./le rfc check` reads it | |
| Message-Authenticator signer | `TestSignMessageAuthenticatorMatchesRFC3579` | |
| EAP-Message framing | `TestEAPMessageSplitsAtTheAttributeLimit` | |
| Challenge loop | `TestRadiusAdminEapChallengeLoopCarriesState` | |
| Wiring from config to wire | `TestRadiusAdminEapReachesTheWire` | |
| Functional proof | `test/plugin/aaa-radius-eap.ci` | done |
| Interop proof | `radius-admin-eap-freeradius` scenario | done |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail closed | An unverified Message-Authenticator discards; it never falls through to a success path |
| Secret handling | The shared secret and the password never reach a log line or an error string |
| State handling | Never interpreted, never persisted, never logged |
| Unbounded work | The round cap and the time budget both hold, and neither can be disabled by the server |
| Downgrade | A server cannot move a configured EAP method back to PAP, and the peer NAKs toward its configured method rather than accepting any offered one |
| Memory | An EAP packet reassembled from attributes is bounded by the RADIUS packet length, which the decoder already bounds |

### Failure Routing
| Failure | Route |
|---------|-------|
| Message-Authenticator mismatch | Silent discard (RFC 3579 Section 3.1); the request stays outstanding |
| Round cap reached | Error from `Authenticate`; the chain tries the next backend |
| Server does not support EAP-Message | It returns Access-Reject (RFC 3579 Section 3.1); the chain stops, as for any reject |
| Unknown `auth-method` word | Refused at config load by the YANG enum |

## Design Insights

The skeleton assumed EAP needed a transport that carries EAP frames, and
concluded a new front end was required. The premise was wrong in both
directions: ze does not relay, it answers, and the peer that answers is already
written and already gated against RFC 3748. What was actually missing was
RADIUS-side plumbing that fits in one file plus a loop.

## Key Design Decisions

| Decision | Why | What it forecloses |
|----------|-----|--------------------|
| Ze acts as the EAP peer | It holds the password, and the login transport carries no EAP | A client-supplied credential ze cannot see, such as a smartcard, cannot be used |
| Move the peer to `internal/core/eap` | It is a leaf with no component dependency; RADIUS must not depend on IKE | A one-commit blast radius across IKE |
| MD5-Challenge and MSCHAPv2 only | Both are password-driven and already implemented | EAP-TLS, which needs an operator certificate and its own config |
| Two enum values, not a method sub-leaf | The credential is one switch with one arm each, which is how RFC 2865 Section 4.1 reads | Negotiating among several acceptable methods |
| Discard an unverified reply | RFC 3579 Section 3.1 says discard | Reporting the mismatch as a rejection, which would stop the chain on a forged packet |

## Known Limitations

- EAP-TLS is implemented in the peer and not offered here. Enabling it is a
  later spec with its own config for the operator certificate and key.
- The peer NAKs toward one configured method. Ze does not negotiate among
  several acceptable methods.

## RFC Documentation (Scope: protocol)
- RFC 3579 Sections 3.1 and 3.2, enrolled by this spec.
- RFC 2865 Section 5.24, already enrolled.
- RFC 3748, already enrolled; this spec adds no EAP-layer behavior and must
  leave that ledger unchanged.

## Discrimination Record (Phase 7)

`ai/rules/interop-and-goal-validation.md` requires a forced RED for each new
proof: break the producing behavior, rebuild the artifact the test drives,
observe the red, restore, observe the green. Every break below was applied
through a Go build overlay, so no file in `internal/component/radius` was
modified on disk: another session held that package.

### `test/plugin/aaa-radius-eap.ci`

Baseline, the fixture run against the isolated binary pair:

```
OK: show bgp ran via RADIUS EAP auth
radius-mock: Access-Request user=admin method=eap eap-type=1 reply=Access-Challenge
radius-mock: Access-Request user=admin method=eap eap-type=26 reply=Access-Challenge
radius-mock: Access-Request user=admin method=eap eap-type=26 reply=Access-Accept
```

**Break 1 -- the State is not returned.** Removed the `AttrState` append from
`eapCredential` (`internal/component/radius/authenticator_eap.go`), which is the
RFC 2865 Section 5.24 obligation AC-7 names, and rebuilt the daemon:

```
radius-mock: Access-Request user=admin method=eap eap-type=1 reply=Access-Challenge
radius-mock: Access-Request user=admin method=eap eap-type=26 reply=Access-Reject
ZE-OBSERVER-FAIL: show bgp via RADIUS EAP auth: exit status 1
```

The mock refuses a round whose State it did not issue, so the second round is
rejected and the login fails. Exit 1.

**Break 2 -- the Message-Authenticator covers the wrong octets.** Changed
`mac.Write(packet)` to `mac.Write(packet[:len(packet)-1])` in
`SignMessageAuthenticator` (`internal/component/radius/packet.go`) and rebuilt:

```
radius-mock: EAP Access-Request discarded user=admin reason="message-authenticator does not verify"
radius-mock: EAP Access-Request discarded user=admin reason="message-authenticator does not verify"
radius-mock: EAP Access-Request discarded user=admin reason="message-authenticator does not verify"
ZE-OBSERVER-FAIL: show bgp via RADIUS EAP auth: exit status 1
```

Three discards, one for each retransmission, then the login times out. That is
the silent discard of RFC 3579 Section 3.1 observed from the server's side, and
it is what makes the signature load bearing here: the mock computes the HMAC
itself (`internal/test/mock/radius/eap.go`, `verifyRequestSignature`) rather
than calling ze's signer.

### `radius-admin-eap-freeradius`

Baseline, `./le --name eapdev integration interop-radius` over all four
scenarios: exit 0.

**Break A -- the State is not returned.** The same overlay as Break 1,
cross-compiled into `test/interop-radius/ze-linux`, packed into the lab image,
and selected with `NO_BUILD=1` so the suite ran that image:

```
── radius-admin-eap-freeradius ──
  ✗ FAIL: assertion 2: radiusop could not run "show version" through RADIUS
    EAP-MSCHAPv2 (exit 1): ssh: handshake failed
```

Assertion 1, the localop control, still passed, so the lab was healthy and the
red is ze's.

**Break B -- ze puts the wrong credential on the wire.** Set `auth-method pap`
in `test/interop-radius/scenarios/radius-admin-eap-freeradius/ze.conf`, with an
unbroken daemon:

```
── radius-admin-eap-freeradius ──
  ✗ FAIL: assertion 1: wait for FreeRADIUS record 'verdict=silent user=localop
    user-password=absent chap-password=absent eap-message=absent
    nas-identifier=ze-interop-nas' timed out before the peer became ready
```

It reds at the FIRST assertion, because the control request already carries the
wrong credential and the checker reads the server's record of what ARRIVED.
Restored, the scenario passes.

### What the FreeRADIUS run found

The scenario failed on its first attempt against a server configuration that
looked correct, and the cause was in the lab rather than in ze:

```
(2) eap: Calling submodule eap_mschapv2 to process data
(2) eap_mschapv2: Auth-Type sub-section not found.  Ignoring.
(2) eap: Sending EAP Failure (code 4) ID 2 length 4
```

`rlm_eap_mschapv2` resolves the section it runs with
`dict_valbyname(PW_AUTH_TYPE, 0, "MSCHAP")` and falls back to `"MS-CHAP"` only
when that misses (`rlm_eap_mschapv2.c:83`, release_3_2_7). FreeRADIUS registers
an Auth-Type value for every module instance it loads, so `mschap` is already
there and the case-insensitive first lookup takes it. The lab's section is
therefore named `Auth-Type mschap`, and `test/interop-radius/site-default`
carries that reasoning beside it.

A second lab finding is why the scenario reads `verdict=state-echo` rather than
a challenge line: FreeRADIUS runs no `post-auth` section for an
Access-Challenge, so the reply that asks the question records nothing. The
`ze_state_echo_log` module writes the round from `authorize` instead, on a
request carrying both an EAP-Message and a State.

The exchange the server logged is the whole conversation, ze's Nak included:

```
(0) eap: Using default_eap_type = MD5
(1) eap: Peer sent packet with method EAP NAK (3)
(1) eap: Found mutually acceptable type MSCHAPv2 (26)
(2) eap: Calling submodule eap_mschapv2 to process data
    Sent Access-Accept
```

## Checklist

### Pre-Spec Verification (before the design is presented)
- [x] The EAP peer's API and its dependency set were read, not inferred.
- [x] The RFC 3579 text was read, not a summary.
- [x] The package placement and the method set were put to the owner and he
      answered.

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 demonstrated
- [ ] Wiring test RED before the loop, GREEN after
- [ ] Interop scenario passes against FreeRADIUS
- [ ] `./le tier check` passes
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Deferral rows naming this spec resolved
- [ ] Citations repointed

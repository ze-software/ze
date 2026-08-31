# Spec: eap-notification-and-nak

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | `plan/spec-rfcgate-6-supported-extraction-signoff.md` (its 2026-08-30 walk produced the six unclassified rfc3748 sites and the scratch sign-off this spec lands) |
| Phase | - |
| Deferral shard | - |
| Handoff | verify |
| Updated | 2026-08-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 3748 Section 5 states, at `rfc/full/rfc3748.txt` line 1487: "All EAP
implementations MUST support Types 1-4, which are defined in this document, and
SHOULD support Type 254." The sentence carries no antecedent. It binds every EAP
implementation, and Ze has one.

Ze supports Type 1 (Identity) and none of Type 2 (Notification), Type 3 (Nak) or
Type 4 (MD5-Challenge). `PeerSession.handleRequest`
(`internal/component/ike/eap/peer.go`) answers an Identity Request and the one
configured method, and returns an error for every other Type. The Nak gap is
reachable today: any authenticator that offers a method Ze does not run gets an
error and a dead IKE SA where the RFC requires a Type-3 Response naming the
method Ze does run.

This spec implements Type 2 and Type 3 on the peer, corrects the authenticator's
handling of a received Nak, lands the requirement rows and both test polarities
for every RFC 3748 site those two types make live, and lands the rfc3748
extraction sign-off that has no honest disposition without them.

Type 4 (MD5-Challenge) is a deliberate deviation, authorized by Thomas on
2026-08-30 and recorded as a row in
`plan/journal/gate-excludes-part-of-its-population.md`. The rationale is RFC 7296
Section 2.16, at `rfc/full/rfc7296.txt` line 2958: "EAP methods that do not
establish a shared key SHOULD NOT be used, as they are subject to a number of
man-in-the-middle attacks". Ze speaks EAP only inside IKEv2, so MD5-Challenge is
a method Ze may never use.

**That rationale does not discharge the obligation, and the spec says so
plainly.** A SHOULD NOT about USING a method is not an answer to a MUST about
SUPPORTING it, and RFC 3748 Section 5.4 closes the one escape it offers. Line
1932 of the same file: "However, if the EAP authenticator can be configured to
authenticate peers locally (e.g., not operate in pass-through), then the
requirement for support of the MD5-Challenge mechanism applies." Ze's
authenticator authenticates locally (`NewSession`,
`internal/component/ike/eap/eap.go`), so the RFC names Ze's case and applies the
requirement to it. This is a real deviation under an owner authorization, not an
obligation that fails to bind.

## Owner Decisions Required

Two questions are the owner's and neither is answerable inside this spec. Both
ask which way to fix, never whether to skip.

### D-1: how an authorized deviation is dispositioned in an extraction sign-off

`rfc/extraction/README.md` publishes a CLOSED set of six exclusion kinds:
`not-a-requirement`, `binds-another-role`, `duplicate-of`, `cross-document`,
`advisory-in-context`, `relocated-to-spec`. **None of them means "an obligation
that binds Ze, that Ze does not meet, by an owner authorization."**

`relocated-to-spec` is the only one that points at an unmet obligation rather
than dismissing its sentence, and it misfits by its own contract: it says the
obligation IS owed by a NAMED spec, and the check refuses it unless that spec
exists on disk and still names the reserved id. An authorized deviation is owed
by nobody, so a spec written to hold it would be a tripwire that never fires,
which is parking with a longer name.

What that implies, stated as the mechanism rather than as a preference:

| Step | Consequence |
|------|-------------|
| Sites `5:2` and `5.4:2` cannot be `excluded` under any kind in the closed set | they must be `mapped` |
| `mapped` requires a requirement id in `rfc/short/rfc3748.md` | a MUST-level row is created |
| A MUST-level row of an enrolled RFC is gated (`Gated`, `internal/le/rfc/rfc.go`) | `evaluate` (`internal/le/rfc/check_core.go`) demands a positive and a negative tagged test |
| No honest test can prove support Ze does not have | the only in-repo release from the polarity duty is a `{gap}` or `{not-applicable}` annotation, which this work is instructed not to propose |

So rfc3748 cannot reach a landed sign-off until D-1 is answered. Three ways to
fix it, with what each costs. The spec picks none.

| Option | What it is | Cost | What it leaves |
|--------|-----------|------|----------------|
| A | Extend the closed set with a kind meaning "owner-authorized deviation", tripwired the way `relocated-to-spec` is: it requires the `plan/journal/` row path, a matching disclosure row in `docs/features/rfc-status.md`, and it counts as an exclusion for the ratchet | a change to `internal/le/rfc` sign-off parsing and checking, `rfc/extraction/README.md`, and tests, as its own spec | the deviation published, the site dispositioned, the ratchet honest |
| B | Use the existing published-divergence route: a `{gap: reason}` annotation on the new row plus the `docs/features/rfc-status.md` disclosure the gap-count check compares against | near zero, and it is what `check_audit` and `checkGapCount` were built for | a `{gap}` in the summary, which `ai/rules/rfc-compliance.md` treats as a doing-less classification the owner must have authorized. He authorized the deviation on 2026-08-30; whether that word extends to this spelling is his to say |
| C | Implement MD5-Challenge as an EAP method | a method, its tests, and a surface no IKEv2 session may select, because RFC3748-7.10-3 already forbids USING a non-key-deriving method there | full support with an unreachable entry point, which is the unwired-feature shape (`ai/rules/completion.md`) |

### D-2: does Ze's authenticator ever SEND a Notification Request

RFC 3748 Section 5.2 makes the send a MAY: "An authenticator MAY send a
Notification Request to the peer at any time when there is no outstanding
Request, prior to completion of an EAP authentication method." A MAY is put to
the owner and is never picked by the implementer (`ai/rules/rfc-compliance.md`).

The answer changes nothing about the mandatory half of this spec, which is
entirely peer-side. It decides whether sites `5.2:4` (the message MUST NOT be
null terminated) and the authenticator half of `5.2:2` become live obligations
with rows and tests, or stay excluded with a false antecedent. Skipping the send
is the smaller surface, and Section 5.2 itself says "In most circumstances,
Notification should not be required."

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - the EAP framework, method dispatch and the result-type traps
  → Decision: a method's last word has its OWN field, `MethodResult.FinalRequest`, because a packet returned in `Response` beside a non-nil `Err` is discarded by `Session.handleMethod`. Any new outcome this spec needs gets a named field, never a flag over an existing one
  → Constraint: the doc's own trap paragraph ("A result type with two fields whose consumer branches on one silently loses the other") governs `PeerResult` as well. A discard expressed as an all-zero `PeerResult` is that defect a third time
- [ ] `docs/architecture/ike/ipsec-11-interop-eap.md` - Ze as the EAP peer, and the strongSwan interop suite
  → Constraint: `test/interop-ipsec/scenarios` is the suite, one directory per scenario holding `swanctl.conf` and `ze.conf`, driven by `./le integration interop-ipsec`. Scenario directories are NAMED and carry no numeric prefix
  → Decision: the peer module (`eap/peer.go`) is separate from the authenticator module (`eap/eap.go`). Type 2 and Type 3 handling belongs to the peer file, and only the Nak RECEIPT path belongs to the authenticator file
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the IKE SA state machine that carries every EAP packet
  → Constraint: `handleEAPResponse` treats a non-nil `PeerResult.Err` as fatal and kills the SA, and sends only when `Response` is non-nil. A silent discard therefore already produces the right WIRE behavior by accident, which is why it needs an explicit outcome rather than a zero value
- [ ] `docs/architecture/core-design.md` - the rfc area as one command
  → Constraint: every check that judges a requirement row lives in `internal/le/rfc`, and a new row is judged the moment it is written. A row cannot land ahead of its two tagged tests

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3748.md` - the summary this spec adds rows to
  → Constraint: requirement ids are never renumbered or removed (`check_retired_requirements`), so every new obligation takes a NEW id appended to the checklist. `RFC3748-5.7-1`, `RFC3748-5-1`, `RFC3748-5.1-1` and `RFC3748-5.1-2` already exist, so the new ids start at `RFC3748-5.2-1` and `RFC3748-5.3.1-1`
  → Constraint: `RFC3748-2.1-3` carries `{single-polarity: positive}` whose stated reason is that Ze "has no NAK-generation path". This spec makes that reason false, so the annotation is removed and the row takes a real negative test in the same phase
- [ ] `rfc/short/rfc7296.md` - EAP inside IKE_AUTH
  → Constraint: Section 2.16 is the carrier. Every EAP packet this spec adds travels in an IKE_AUTH exchange, so a peer that answers nothing stalls until the IKEv2 retransmit timer fires rather than failing
- [ ] `rfc/full/rfc3748.txt` - Sections 2.1, 5, 5.2, 5.3.1, 5.3.2, 5.4, 5.7 read in full
  → Decision: Section 5.3.1 bounds the Nak to a Request "for an unacceptable authentication Type (4-253,255)" or a Type 254 Request the peer cannot interpret. Types 1, 2 and 3 are outside that range, so an unwanted Identity or Notification Request is never Nak'd
  → Decision: Section 5.7 sends a peer that cannot interpret an Expanded Type to Section 5.3.1, which is the LEGACY Nak. Ze therefore never encodes an Expanded Nak, and Section 5.3.2's composition rules stay bound to a role Ze does not play
  → Constraint: Section 2.1's "A peer MUST NOT send a Nak (legacy or expanded) in reply to a Request after an initial non-Nak Response has been sent" is reconciled with Section 5.3.1 by Section 5.4's own sentence, "The Response MAY be either of Type 4 (MD5-Challenge), Nak (Type 3), or Expanded Nak (Type 254)", which describes a Nak sent after an Identity Response. The initial non-Nak Response is therefore the peer's first Response to an authentication METHOD Request (Type 4 or greater), not the Identity Response
- [ ] `rfc/full/rfc7296.txt` - Section 2.16 read in full
  → Constraint: line 2958 carries the MD5-Challenge rationale verbatim, and it is a SHOULD NOT about USE. It is quoted in the journal row and in this spec, and it is never presented as discharging the Section 5 MUST
- [ ] `rfc/extraction/README.md` - the sign-off artifact, its closed sets and its ratchets
  → Constraint: the exclusion-kind set is CLOSED and holds six kinds, none meaning an authorized deviation. See D-1
  → Constraint: a refresh carries decisions forward from the LANDED artifact only, and rfc3748 has none. Re-running `./le rfc extraction-create` over the scratch file destroys the classification, so the scratch artifact is edited in place and never regenerated

**Key insights:** (minimal context to resume after compaction)
- Six rfc3748 sites are unclassified in the scratch artifact: `5:2`, `5.2:1`, `5.2:5`, `5.3.1:3`, `5.4:2`, `5.7:2`. Four are closed by this spec's code; `5:2` and `5.4:2` are the Type-4 pair D-1 governs
- Landing the Nak makes several ALREADY excluded sites live, because their exclusion reason is "Ze composes none". Each needs a new row and two tests, not just the two that were unclassified
- Two committed tests assert the non-conformant behavior and are corrected here, not preserved: `TestRFC3748PeerNeverSendsNAK` (`rfc3748_test.go`) and the `RFC3748-4.1-5` negative arm of `TestRFC3748ResponseTypeMatchesRequest` (`rfc3748_walk_test.go`)
- `MethodResult.FinalRequest` (uncommitted in the tree on 2026-08-30, from the concurrent RFC 2759 Failure work) is the authenticator's "request then terminate" outcome. The peer's mandatory Notification work never touches it. See Key Design Decisions

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/eap/peer.go` - `PeerSession.Process` counts a round, refuses a parked error and a terminal state, then switches on Code. `handleRequest` answers Identity with Identity in the identity state, dispatches the configured method, and returns an error for every other Type in both states. No Type-2 and no Type-3 path exists
- [ ] `internal/component/ike/eap/eap.go` - `Packet.Encode` and `DecodePacket` are Type-agnostic, so a Type-2 or Type-3 packet needs no codec work. `TypeNAK` and `TypeExpandedEAP` are declared constants with no producer. There is no `TypeNotification` constant. `Session.handleIdentity` and `Session.handleMethod` answer a received `TypeNAK` Response with `s.failure(response)` and discard the desired Types the Nak carried
- [ ] `internal/component/ike/engine/fsm.go` - `handleEAPResponse` reads `PeerResult`: a non-nil `Err` logs and sets `StateDead`, `Done` builds the AUTH payload from the MSK, a non-nil `Response` is sent, and an all-zero result silently sends nothing and waits for the retransmit timer
- [ ] `internal/component/ike/eap/rfc3748_test.go` - `TestRFC3748PeerNeverSendsNAK` carries the `RFC3748-2.1-3` positive tag and asserts, in its own words, that the peer "errors (never NAKs) on an unexpected type" and "must error instead"
- [ ] `internal/component/ike/eap/rfc3748_walk_test.go` - the ten rows the 2026-08-30 walk added, each with both polarities. Tag format is `// RFC requirement: <RID> positive|negative -- <prose>`. Its `RFC3748-4.1-5` negative arm asserts that a Type-99 Request produces `res.Err != nil` and no Response
- [ ] `rfc/short/rfc3748.md` - the checklist. `RFC3748-2.1-3` carries `{single-polarity: positive}` justified by the absence of a NAK path. `RFC3748-5.7-1` carries `{not-applicable}` justified by the absence of any Expanded Type handling
- [ ] `docs/features/rfc-status.md` - line 234 reads "RFC 3748, EAP, Supported in IPsec" with "No tracked gap in current source anchors"
- [ ] `test/interop-ipsec/scenarios/eap-mschapv2/swanctl.conf` - the strongSwan side names one method, `remote { auth = eap-mschapv2 }`, with an `eap-testuser` secret
- [ ] `tmp/session/2026-08-30-7f961064-45bd-4f4c-9e8f-83e986058d66/scratch/rfc-extraction/rfc3748.json` - the unlanded sign-off. 103 sites, 57 sections, register `rfc2119`, six sites with a null disposition. Its path is recorded exactly in the Files to Modify section

**Behavior to preserve:** (unless the user explicitly said to change it)
- `PeerResult` keeps its meaning for `Done`, `MSK`, `Err` and `Response`, and `handleEAPResponse` keeps treating a non-nil `Err` as fatal
- The round cap `maxEAPRounds` keeps counting every Request, Notification Requests included
- The peer keeps answering the configured method's Requests unchanged, so `eap-mschapv2`, `eap-tls` and `eap-tls13` interop scenarios are untouched
- `Packet.Encode` stays the single encoder. A Nak and a Notification Response are ordinary Type or Type-Data packets and get no encoder of their own

**Behavior to change:** (only what the user asked for)
- A Type-2 Request is answered with a Type-2 Response instead of an error, in both peer states, without advancing the peer state
- A Request for an unacceptable authentication Type (4-253, 255), and a Type-254 Request, are answered with a legacy Nak instead of an error, until the peer has sent its first non-Nak Response to a method Request
- After that point, a Request of an unexpected Type is discarded rather than errored, and the discard is an explicit outcome rather than a zero value
- The authenticator records the desired Types a received Nak carried, so the operator sees which method the peer asked for

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An IKE_AUTH message arrives on the IKEv2 UDP transport carrying an encrypted EAP payload, while the SA is in `StateEAPInProgress`. `handleEAPResponse` (`internal/component/ike/engine/fsm.go`) decrypts it and calls `wireEAPToPacket`, which yields Code, Identifier, Type and Type-Data
- Format at entry: one EAP packet, Code 1 (Request), Type 2, 3, 4, 254 or any other value

### Transformation Path
1. `handleEAPResponse` calls `PeerSession.Process`, which counts the round and switches on Code
2. `Process` calls `handleRequest`, which now inspects the Type BEFORE dispatching to a method: Type 2 goes to the Notification path, an unacceptable authentication Type goes to the Nak path, and everything else keeps its current route
3. The Notification path builds a Type-2 Response with zero-length Type-Data and the Request's Identifier, and reports the displayable message to the caller
4. The Nak path builds a Type-3 Response whose Type-Data is one octet naming the configured method, with the Request's Identifier
5. The discard path returns an explicit discard outcome with no packet
6. `handleEAPResponse` sends the Response through `sendEAPResponsePacket`, logs a Notification message, or logs a discard and leaves the SA alive for the retransmit timer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IKE engine ↔ EAP peer | `PeerResult` struct, one call per received packet | No |
| EAP peer ↔ wire | `Packet.Encode`, unchanged and Type-agnostic | No |
| Ze peer ↔ strongSwan authenticator | EAP Type-3 Nak inside an IKE_AUTH exchange | No |
| Summary ↔ tests | `// RFC requirement: <RID> positive\|negative` tags read by `internal/le/rfc` | No |

### Integration Points
- `PeerSession.handleRequest` - the one dispatcher every Request already passes through, so the two new types are handled there and nowhere else
- `Session.handleIdentity` and `Session.handleMethod` - the authenticator's two receive paths for a Nak Response, which already branch on `TypeNAK` and only need the desired Types recorded
- `handleEAPResponse` - the only consumer of `PeerResult`, which gains one branch for the discard outcome and one log line for a Notification message

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
| A-1 | "an initial non-Nak Response" in RFC 3748 Section 2.1 means the peer's first Response to an authentication METHOD Request, not the Identity Response | RFC 3748 Section 5.4: "The Response MAY be either of Type 4 (MD5-Challenge), Nak (Type 3), or Expanded Nak (Type 254)", which describes a Nak sent after an Identity Response. The literal alternative makes Section 5.3.1's MUST unreachable in every conversation | the Nak would be forbidden the moment the peer answers the Identity Request, which no implementation could satisfy | the reading is stated in the code comment above the guard, and `TestRFC3748PeerNaksBeforeItCommitsToAMethod` drives both sides of the boundary | unvalidated |
| A-2 | The strongSwan image built from `test/interop-ipsec/Dockerfile.strongswan` (alpine 3.21, package `strongswan`) ships the `eap-dynamic` plugin | the Alpine package is the full charon build, and the scenario needs a peer that reacts to a Nak by offering another method | the scenario falls back to the single-method form: strongSwan offers `eap-md5` only, Ze Naks, and the assertion is strongSwan's "received EAP Nak" log line plus no tunnel | `docker run --rm <image> ls /usr/lib/ipsec/plugins` before the scenario is written | unvalidated |
| A-3 | An all-zero `PeerResult` today means "send nothing and wait", so an explicit discard outcome changes no wire behavior | `handleEAPResponse` (`internal/component/ike/engine/fsm.go`) guards the send with `if result.Response != nil` and has no else branch | the discard would need a carrier change as well as a peer change | `TestPeerDiscardLeavesTheSAAlive` drives the engine path and asserts the SA state is unchanged | unvalidated |
| A-4 | Neither EAP-MSCHAPv2 nor EAP-TLS prohibits Notification messages in its own specification | RFC 3748 Section 5.2's excusing clause requires the METHOD specification to say so, and neither RFC 2759 nor RFC 5216 contains such a prohibition | the peer would owe a silent discard rather than a Response while that method runs, which is site `5.2:3` | a read of RFC 2759 and RFC 5216 for a Notification prohibition, recorded in the site's reason | unvalidated |
| A-5 | `Session.err` is the right home for the desired Types a received Nak carried | `Session.Err()` already exists for exactly this reason: RFC 3748 Section 4.2 leaves an EAP-Failure no field to carry a reason, and `handleResponderEAP` logs it | the information would need a second channel out of the authenticator | `TestAuthenticatorRecordsTheTypesANakAskedFor` reads `Session.Err()` after a driven Nak | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A Notification Request is unauthenticated and repeatable, so an authenticator can drive the peer round the exchange indefinitely | an EAP exchange that never terminates | `Process` counts every Request against `maxEAPRounds` before dispatch, Notification Requests included. `TestNotificationRequestsCountAgainstTheRoundCap` pins it |
| R-2 | Answering a Notification Request mid-method corrupts the method's pending state, breaking EAP-MSCHAPv2 or EAP-TLS | an interop scenario that regresses | the Notification path returns before any method dispatch and touches no method field. `TestNotificationMidMethodLeavesTheMethodUntouched` drives a full MSCHAPv2 exchange with a Notification injected between rounds |
| R-3 | The Nak guard is placed so that a Nak escapes after the peer has committed to a method, violating Section 2.1 | no red test, because today's tests assert the opposite behavior | the two tests that assert the current behavior are corrected FIRST, in phase 1, so the boundary has a test before the code moves |
| R-4 | The discard outcome is read by a future caller as "nothing happened", reintroducing the zero-value defect the architecture doc already names twice | a caller that branches on `Response == nil` alone | the outcome is a named field on `PeerResult` with a doc comment, and `handleEAPResponse` logs it. A reviewer checks that no path returns a bare `PeerResult{}` |
| R-5 | Landing the Nak silently changes the meaning of sites already excluded as "Ze composes none", leaving the sign-off arithmetic honest-looking and wrong | the sign-off passes with stale reasons | phase 5 re-reads every 5.2, 5.3.1, 5.3.2, 5.4 and 5.7 site reason against the new code, and the phase's deliverable is the list of re-classified sites |
| R-6 | strongSwan answers a Nak by tearing the SA down rather than offering another method, so the interop scenario proves less than it claims | the scenario passes without a second method being offered | the scenario asserts the Nak on the wire from strongSwan's own log, which is a fact about Ze's bytes whatever strongSwan does next |
| R-7 | The rfc3748 sign-off is blocked on D-1, so the spec cannot close | phase 6 has nothing to land | the spec stops at phase 6 and reports. It does not invent a kind, and it does not annotate the row to make the gate green |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An EAP-authenticated IPsec client fails to establish, or establishes against an authenticator it should have refused. The blast radius is road-warrior IKEv2 sessions using EAP, and nothing else: no BGP, no CLI, no config path reads this code |
| How is it reverted? | Single commit revert. The wire change is a Response Ze did not previously send, so nothing persists and no peer state survives a restart |
| Who else touches this path? | A concurrent session is editing `peer.go`, `eap.go` and `eap_mschapv2.go` for the RFC 2759 Failure packet, adding `MethodResult.FinalRequest`. That work is uncommitted at the time of writing and must be landed first, or merged carefully: both change `handleRequest`'s neighborhood |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH message carrying an EAP Request of Type 4 while the SA is in `StateEAPInProgress` | → | `handleEAPResponse` → `PeerSession.Process` → `handleRequest` → the Nak path | `TestEngineSendsANakForAnUnacceptableType` |
| IKE_AUTH message carrying an EAP Request of Type 2 | → | `handleEAPResponse` → `PeerSession.Process` → `handleRequest` → the Notification path | `TestEngineAnswersANotificationRequest` |
| IKE_AUTH message carrying an EAP Request of an unexpected Type after the method has begun | → | `handleEAPResponse` → `PeerSession.Process` → `handleRequest` → the discard path | `TestPeerDiscardLeavesTheSAAlive` |
| strongSwan offering a method Ze does not run | → | the same chain, over the wire, in a container | `eap-nak-method-negotiation` scenario under `./le integration interop-ipsec` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer in the identity state receives a Request with Type 2 and a displayable message | It answers Code 2, Type 2, Type-Data of zero octets, Identifier equal to the Request's. Total packet length is 5 octets |
| AC-2 | Peer mid-method receives a Request with Type 2, then the next method Request | The Notification Response goes out, the peer state and every method field are unchanged, and the following method Request is answered exactly as it would have been without the Notification |
| AC-3 | Peer receives a Request with Type 2 carrying a message | The message reaches the caller through `PeerResult` and the engine logs it once, with the peer name |
| AC-4 | Peer receives a Request with Type 2 carrying a zero-length or non-UTF-8 message | It still answers with a Type-2 Response and never with a Type-3 Nak |
| AC-5 | Peer configured for EAP-MSCHAPv2, in the identity state or having answered only the Identity Request, receives a Request with Type 40 | It answers Code 2, Type 3, Identifier equal to the Request's, Type-Data one octet holding 26. Total packet length is 6 octets |
| AC-6 | The same peer receives a Request with Type 254 | It answers a LEGACY Nak: Code 2, Type 3, one octet naming the configured method. It never emits a Type-254 packet |
| AC-7 | The same peer receives a Request with Type 4 (MD5-Challenge) | It answers a legacy Nak naming the configured method, so an MD5-Challenge offer is refused by the protocol's own mechanism rather than by an error |
| AC-8 | Peer has answered a method Request with a non-Nak Response, then receives a Request with Type 40 | No Response and no error. The result carries the explicit discard outcome, the SA stays alive, and the engine logs the discard |
| AC-9 | Peer has answered a method Request, then receives an Identity Request | Discarded on the same path as AC-8. Type 1 is outside the 4-253 and 255 range, so no Nak is sent |
| AC-10 | Authenticator receives a Type-3 Nak Response carrying the octets 13 and 5 | It ends the exchange with an EAP-Failure, and `Session.Err()` names the Types the peer asked for |
| AC-11 | Any Nak this spec produces, inspected on the wire | Length field reads 6 for one desired Type, and the desired Type octet is never 0 while a configured method exists |
| AC-12 | `rfc/short/rfc3748.md` after this work | It carries the new rows named in the Files to Modify section, each with a positive and a negative tagged test, and `RFC3748-2.1-3` carries no `{single-polarity}` annotation |
| AC-13 | `./le rfc check` over the tree | Passes, with no unknown requirement, no missing polarity and no stale annotation for rfc3748 |
| AC-14 | `./le integration interop-ipsec` with the new scenario | strongSwan's log records an EAP Nak received from Ze, and the scenario is RED when the Nak path is reverted and the Ze image rebuilt |
| AC-15 | `rfc/extraction/rfc3748.json` | Every site carries a disposition, and the artifact lives under `rfc/extraction/`. Blocked on D-1 for sites `5:2` and `5.4:2`, and the spec reports rather than inventing a kind |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Connects a road-warrior client to an IKEv2 concentrator that offers EAP-MD5 first and EAP-MSCHAPv2 second | wire → IKE_AUTH → `handleEAPResponse` → `PeerSession.handleRequest` → Nak naming Type 26 → concentrator switches → tunnel establishes | `eap-nak-method-negotiation` interop scenario |
| 2 | Connects to a concentrator that sends a password-expiry notice mid-authentication | wire → IKE_AUTH → `handleRequest` → Notification Response, message logged → the method continues and the tunnel establishes | `TestNotificationMidMethodLeavesTheMethodUntouched` |
| 3 | Reads the daemon log after a failed EAP connection where the concentrator offered only a method Ze does not run | authenticator or peer log line naming the Types asked for and the Type offered | `TestAuthenticatorRecordsTheTypesANakAskedFor` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerAnswersANotificationRequest` | `internal/component/ike/eap/rfc3748_notification_test.go` | AC-1, AC-3. Tags `RFC3748-5.2-1` positive | |
| `TestPeerNeverNaksANotificationRequest` | `internal/component/ike/eap/rfc3748_notification_test.go` | AC-4. Tags `RFC3748-5.2-1` negative and `RFC3748-5.2-2` positive | |
| `TestNotificationMidMethodLeavesTheMethodUntouched` | `internal/component/ike/eap/rfc3748_notification_test.go` | AC-2, R-2 | |
| `TestNotificationRequestsCountAgainstTheRoundCap` | `internal/component/ike/eap/rfc3748_notification_test.go` | R-1 | |
| `TestPeerNaksAnUnacceptableAuthenticationType` | `internal/component/ike/eap/rfc3748_nak_test.go` | AC-5, AC-7, AC-11. Tags `RFC3748-5.3.1-1` positive and `RFC3748-5.3.1-2` positive | |
| `TestPeerNaksAnExpandedTypeRequestWithALegacyNak` | `internal/component/ike/eap/rfc3748_nak_test.go` | AC-6. Tags `RFC3748-5.3.1-1` negative, the Expanded arm | |
| `TestNakIdentifierMatchesTheRequest` | `internal/component/ike/eap/rfc3748_nak_test.go` | AC-5. Tags `RFC3748-5.3.1-3` positive and negative | |
| `TestPeerDoesNotNakAMethodError` | `internal/component/ike/eap/rfc3748_nak_test.go` | Tags `RFC3748-5.3.1-4` positive and negative: a malformed method Request produces an error, never a Nak used as an error indication | |
| `TestRFC3748PeerNaksBeforeItCommitsToAMethod` | `internal/component/ike/eap/rfc3748_nak_test.go` | AC-8, A-1. Replaces the deleted body of `TestRFC3748PeerNeverSendsNAK`, tagging `RFC3748-2.1-3` positive and negative | |
| `TestPeerDiscardsAnIdentityRequery` | `internal/component/ike/eap/rfc3748_nak_test.go` | AC-9 | |
| `TestAuthenticatorRecordsTheTypesANakAskedFor` | `internal/component/ike/eap/rfc3748_nak_test.go` | AC-10, A-5 | |
| `TestRFC3748ResponseTypeMatchesRequest` (corrected) | `internal/component/ike/eap/rfc3748_walk_test.go` | `RFC3748-4.1-5`: its negative arm now asserts a Nak Type rather than an error | |
| `TestEngineAnswersANotificationRequest` | `internal/component/ike/engine/eap_wiring_test.go` | Wiring row 2 | |
| `TestEngineSendsANakForAnUnacceptableType` | `internal/component/ike/engine/eap_wiring_test.go` | Wiring row 1 | |
| `TestPeerDiscardLeavesTheSAAlive` | `internal/component/ike/engine/eap_wiring_test.go` | AC-8, A-3, R-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Request Type that earns a Nak | 4-253 and 255 | 253 | 3 (Nak itself, never in a Request) | 254 (Expanded, earns a legacy Nak by Section 5.7 rather than by the 4-253 rule) |
| Request Type handled without a Nak | 1-3 | 3 | N/A | 4 |
| Nak packet Length with one desired Type | 6 and up | 6 | 5 (a Nak with no Type-Data) | N/A |
| Notification Response Length | 5 exactly | 5 | 4 (no Type field) | 6 (Type-Data must be zero octets) |
| Rounds consumed by Notification Requests | 1 to `maxEAPRounds` | 20 | N/A | 21 refuses with `ErrTooManyRounds` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-eap-nak-unacceptable-type` | `test/ipsec/ipsec-eap-nak-unacceptable-type.ci` | An operator reads the daemon log after a concentrator offered a method Ze does not run, and sees the Type Ze asked for instead of an unexplained dead SA | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `eap-nak-method-negotiation` | `test/interop-ipsec/scenarios/` | strongSwan | Ze's Type-3 Nak is a packet another implementation parses as a Nak, and the method it names is the one Ze runs. Proven RED by reverting the Nak path and rebuilding `test/interop-ipsec/ze-linux` | |

## Files to Modify
- `internal/component/ike/eap/peer.go` - the Notification path, the Nak path, the Section 2.1 boundary, the discard outcome, and the `PeerResult` fields that carry them
- `internal/component/ike/eap/eap.go` - a `TypeNotification` constant, and the authenticator recording the desired Types a received Nak carried
- `internal/component/ike/engine/fsm.go` - `handleEAPResponse` logs the Notification message and the discard, and keeps the SA alive across a discard
- `internal/component/ike/eap/rfc3748_test.go` - `TestRFC3748PeerNeverSendsNAK` asserts the behavior this spec corrects. Its body is replaced and its tag moves to the new test
- `internal/component/ike/eap/rfc3748_walk_test.go` - the `RFC3748-4.1-5` negative arm asserts an error for an unhandled Type
- `rfc/short/rfc3748.md` - six new rows and the removal of the `RFC3748-2.1-3` single-polarity annotation
- `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - the EAP framework doc, whose RFC obligations list gains Types 2 and 3 and the authorized Type-4 deviation
- `docs/architecture/ike/ipsec-11-interop-eap.md` - the peer doc, whose Proof section gains the new scenario
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the engine doc, for the discard outcome the SA now survives
- `docs/architecture/core-design.md` - named because the rfc area is one command and this spec adds rows its checks judge. Unaffected in content unless D-1 chooses option A
- `docs/features/rfc-status.md` - the RFC 3748 row, which today claims no tracked gap
- `docs/features.md` - the EAP feature entry
- `plan/journal/gate-excludes-part-of-its-population.md` - the authorized-deviation row, written with this spec rather than at closure

## Files to Create
- `internal/component/ike/eap/rfc3748_notification_test.go` - the Type-2 tests
- `internal/component/ike/eap/rfc3748_nak_test.go` - the Type-3 tests and the Section 2.1 boundary
- `internal/component/ike/engine/eap_wiring_test.go` - the three engine-level wiring tests
- `test/ipsec/ipsec-eap-nak-unacceptable-type.ci` - the operator-visible functional test
- `test/interop-ipsec/scenarios/eap-nak-method-negotiation/swanctl.conf` - strongSwan offering a method Ze does not run
- `test/interop-ipsec/scenarios/eap-nak-method-negotiation/ze.conf` - Ze configured for EAP-MSCHAPv2
- `rfc/extraction/rfc3748.json` - the sign-off, moved from this session's scratch once every site carries a disposition

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No operator-visible option. Which EAP method Ze runs is already configured by `authentication { mode ... }`, and the Nak names that method |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No command changes. The behavior is observed through the daemon log and through the tunnel coming up |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-eap-nak-unacceptable-type.ci` |
| Pipe completeness | N-A | No command output added |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or certificate. The change is confined to bytes inside an existing IKE_AUTH exchange |
| Prometheus counters/metrics | No | The EAP package publishes no counters today, and adding a metric surface for a refusal path is a separate decision. The refusal is visible in the log line the engine writes |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, EAP entry: Ze now negotiates the method by Nak rather than failing |
| 2 | Config syntax changed? | No | No new syntax. Verified by the absence of a new YANG leaf in the Integration Checklist |
| 3 | CLI command added/changed? | No | No command touched |
| 4 | API/RPC added/changed? | No | No RPC touched |
| 5 | Plugin added/changed? | N-A | The IKE component is not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`, which describes the EAP modes an operator can configure and now owes a sentence on what happens when the far end offers another one |
| 7 | Wire format changed? | Yes | `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` carries the EAP packet shapes. The Nak and Notification Response formats land there with their RFC sections |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc3748.md` and the `docs/features/rfc-status.md` RFC 3748 row, which must stop claiming no tracked gap while the Type-4 deviation stands |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new `.ci`, and `docs/architecture/testing/interop.md` if the interop suite gains a scenario the page enumerates |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares feature presence, and EAP support is already listed |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-7-ikev2-engine.md` for the discard outcome, `docs/architecture/ike/ipsec-11-interop-eap.md` for the peer |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-eap-notification-and-nak.md` and name every document it lists. `peer.go` declares `ipsec-11-interop-eap.md`, `eap.go` declares `ipsec-9-ikev2-eap-nat.md`, `fsm.go` declares `ipsec-7-ikev2-engine.md`, and all three are named above |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` shows the EAP peer configuration. The examples stay valid because no syntax changes; verify rather than edit |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - make the two new types reachable from the engine, with failing tests
   - Tests: `TestEngineAnswersANotificationRequest`, `TestEngineSendsANakForAnUnacceptableType`, `TestPeerDiscardLeavesTheSAAlive`
   - Files: `internal/component/ike/engine/eap_wiring_test.go`, `internal/component/ike/eap/peer.go` (stubs only), `internal/component/ike/eap/eap.go` (`TypeNotification`)
   - Verify: the three tests fail because the peer still errors. The engine path is exercised end to end, so a later green proves reachability rather than a helper working alone
2. **Phase: Correct the tests that assert the current behavior** - before any behavior moves
   - Tests: `TestRFC3748PeerNeverSendsNAK` (`rfc3748_test.go`) and the `RFC3748-4.1-5` negative arm of `TestRFC3748ResponseTypeMatchesRequest` (`rfc3748_walk_test.go`)
   - Files: those two test files
   - Verify: both now assert the RFC's behavior and are RED. Neither is deleted, and no assertion is weakened: the Section 2.1 half of the old test becomes the positive arm of `TestRFC3748PeerNaksBeforeItCommitsToAMethod`
3. **Phase: Notification (Type 2)** - the peer's mandatory Response
   - Tests: the four tests in `rfc3748_notification_test.go`
   - Files: `internal/component/ike/eap/peer.go`, `internal/component/ike/engine/fsm.go`
   - Verify: a Type-2 Request is answered in both peer states, the method's state is untouched, and the message reaches the engine log
4. **Phase: Nak (Type 3)** - the peer's mandatory Response, the Section 2.1 boundary and the discard
   - Tests: the seven tests in `rfc3748_nak_test.go`, plus the corrected pair from phase 2
   - Files: `internal/component/ike/eap/peer.go`, `internal/component/ike/eap/eap.go`, `internal/component/ike/engine/fsm.go`
   - Verify: every test from phases 2 and 4 is green, and the three interop scenarios that already exist still pass
5. **Phase: Requirement rows, tags and documentation**
   - Tests: `./le rfc check`
   - Files: `rfc/short/rfc3748.md`, the three architecture docs, `docs/features.md`, `docs/guide/ipsec.md`, `docs/features/rfc-status.md`, `docs/functional-tests.md`
   - Verify: each new row has both polarities, `RFC3748-2.1-3` has no annotation left, and every doc edit lands in this phase rather than at closure
6. **Phase: Functional and interop proof**
   - Tests: `test/ipsec/ipsec-eap-nak-unacceptable-type.ci`, the `eap-nak-method-negotiation` scenario
   - Files: the `.ci` and the two scenario files
   - Verify: the interop scenario is proven RED by reverting the Nak path and rebuilding `test/interop-ipsec/ze-linux`, then green with it restored. The RED output is recorded
7. **Phase: rfc3748 sign-off**
   - Tests: `./le rfc check`, `./le rfc extraction-status`
   - Files: `rfc/extraction/rfc3748.json`, moved from the scratch path named in Current Behavior
   - Verify: sites `5.2:1`, `5.3.1:3` become `mapped`; `5.2:5`, `5.7:2` and `5.4:1` become `excluded` with kind `duplicate-of` naming the ids the mapped sites carry; every 5.2, 5.3.1 and 5.4 site whose exclusion reason reads "Ze composes none" is re-read against the new code and re-classified or given a corrected reason. Sites `5:2` and `5.4:2` wait on D-1: if the answer has not arrived, the artifact stays in scratch, the spec reports, and no kind is invented and no annotation is written to make the gate green

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and AC-15 states its blocked status rather than claiming a landing |
| Feature completeness | Both new types are reachable from `handleEAPResponse`, not only from a unit test that calls `handleRequest` |
| Correctness | The Nak fires only for Types 4-253, 255 and 254, and only before the peer's first non-Nak Response to a method Request. Types 1, 2 and 3 never earn a Nak |
| Correctness | The Notification path returns before every method dispatch, so no method field is read or written on that path |
| Naming | `TypeNotification` matches the constant style of `TypeIdentity` and `TypeNAK`. The discard field on `PeerResult` names the outcome, not the mechanism |
| Data flow | The peer holds no logger. Every diagnosis, the Notification message included, leaves through `PeerResult` and is logged by the engine |
| Rule: `ai/rules/principles.md` | No path returns a bare `PeerResult{}`. A discard is an explicit outcome a caller must branch on, never a zero value that reads as "nothing happened" |
| Rule: `ai/rules/no-layering.md` | The authenticator's terminate-after-request outcome is `MethodResult.FinalRequest` and stays the only one. This spec adds no second mechanism for it |
| Rule: `ai/rules/rfc-compliance.md` | Every new MUST enforced in code carries a comment naming the RFC section and quoting the requirement, and the Type-4 deviation is never described as an obligation that does not bind |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The peer answers Type 2 and Type 3 | `go test ./internal/component/ike/eap/ -run 'RFC3748(Notification\|Nak\|Peer)'` |
| Both types are reachable from the engine | `go test ./internal/component/ike/engine/ -run 'TestEngine(Answers\|Sends)'` |
| Six new rows with both polarities | `./le rfc check`, and `grep -c 'RFC3748-5.2-\|RFC3748-5.3.1-' rfc/short/rfc3748.md` |
| No stale single-polarity annotation | `grep 'RFC3748-2.1-3' rfc/short/rfc3748.md` shows no `{single-polarity` |
| The interop scenario exists and is named, not numbered | `ls test/interop-ipsec/scenarios/eap-nak-method-negotiation` |
| The interop scenario discriminates | the recorded RED output from the revert-and-rebuild run |
| The journal row is written | `grep 'MD5-Challenge' plan/journal/gate-excludes-part-of-its-population.md` |
| The sign-off lands or the block is reported | `./le rfc extraction-status` names rfc3748 as signed, or the spec's phase 7 records D-1 as unanswered |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A Notification Request arrives unauthenticated, before any key is derived. Its Type-Data is attacker-controlled and reaches a log line, so it is logged as data and never used to build a path, a command or a format string |
| Resource exhaustion | Notification Requests are free for an attacker to repeat. They must count against `maxEAPRounds`, and the log line must not be unbounded in size |
| Error leakage | The Nak names the method Ze is configured for. That is the negotiation the RFC requires and discloses nothing an authenticator does not already learn from a successful exchange |
| Authorization that could fail open | The discard path must not be reachable in a way that ends the exchange as a success. It returns no `Done` and no MSK, and the SA stays in `StateEAPInProgress` until the carrier times out |
| Downgrade | A Nak must never name Type 4, Type 5 or Type 6. Ze asks for the method it runs, so an attacker cannot use the Nak to steer Ze onto a non-key-deriving method that RFC3748-7.10-3 forbids |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| `./le rfc check` refuses a new row | The row is missing a polarity or carries a stale annotation. Never annotate to silence it |
| The concurrent RFC 2759 work conflicts in `peer.go` | Land that work first, then rebase this one. Do not duplicate its outcome |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The extraction walk's value shows here as a mechanism rather than a claim. Six sites had no honest disposition, and four of them named a defect a reader of the summary could not have seen: `rfc/short/rfc3748.md` declared no obligation about Type 2 or Type 3 at all, so the gate was green over an unimplemented MUST for as long as the summary existed
- An exclusion reason that says "Ze composes none" is a claim about today's code, so it goes stale the moment the code changes. The sign-off has no mechanism that notices, which makes phase 7's re-read of the sibling sites a required step rather than diligence
- The peer's refusal path had one shape for three different situations: an unacceptable method, an out-of-order Request and a malformed method payload all returned an error. The RFC gives each a different answer, which is why the fix is three paths and not one condition

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The Notification work does NOT build on `MethodResult.FinalRequest` | reusing the concurrent RFC 2759 outcome for the Notification Request | They sit on opposite sides of the exchange. `MethodResult` is the AUTHENTICATOR's method result, read by `Session.handleMethod`; the mandatory Notification duty is the PEER's, produced by `PeerSession.handleRequest`, which never constructs a `MethodResult`. There is nothing to reuse and nothing to duplicate |
| If D-2 says Ze's authenticator DOES send Notification Requests, the terminal case uses `MethodResult.FinalRequest` unchanged | a second terminate-after-request field | A warning sent immediately before a refusal is exactly "send this, then fail with Err", which is what `FinalRequest` already means. A second mechanism for one meaning is the layering this repository forbids. The non-terminal case, a notice mid-exchange that expects a Notification Response and continues, is `Response` carrying a Type-2 packet plus the state to remember it, and it is not `FinalRequest` |
| Ze sends a LEGACY Nak for a Type-254 Request | implementing the Expanded Nak of Section 5.3.2 | Section 5.7 sends a peer not equipped to interpret the Expanded Type to Section 5.3.1, which is the legacy Nak, and Section 5.3.2 says the Expanded Nak "MUST be sent only in reply to a Request of Type 254 ... where the authentication Type is unacceptable" by a peer that supports Expanded Types. Section 5 puts that support at SHOULD. Implementing the Expanded Nak would mean claiming Expanded support Ze does not have |
| The discard is an explicit `PeerResult` outcome | returning a zero `PeerResult`, or returning an error | A zero result already produces the correct wire behavior, which is exactly what makes it dangerous: it is indistinguishable from a bug at every call site, and the same architecture doc records this defect twice in this package. An error is wrong because Section 2.1 says discard, and `handleEAPResponse` kills the SA on an error |
| The Nak names the one configured method, never Type 0 | sending Type 0 (no viable alternative) | Type 0 tells the authenticator to stop (Section 5.3.1: it "SHOULD NOT send another Request"). Ze always has a viable alternative, which is the method the operator configured, and naming it is what makes the negotiation work |
| The Type-4 deviation is recorded in `plan/journal/gate-excludes-part-of-its-population.md` | a new class file named for the deviation | The class the row belongs to is the durable one: an obligation that binds Ze sat outside every checked population, first because the summary declared no row and then because the walk excluded it on circular grounds. Two rows from the same walk already sit in that file. `ai/rules/rfc-compliance.md` requires the row; it does not require a class of its own |

## Known Limitations

- Type 4 (MD5-Challenge) is not implemented. This is the authorized deviation of 2026-08-30 and it is published in the journal row, not hidden
- Type 254 (Expanded Types) is not implemented. RFC 3748 Section 5 puts it at SHOULD, and Ze answers a Type-254 Request with the legacy Nak the RFC prescribes for a peer that lacks the support
- Ze's AUTHENTICATOR still offers exactly one method, so a received Nak ends the exchange. Offering a second method after a Nak is a feature with its own configuration surface and is not in this spec
- Whether Ze's authenticator ever SENDS a Notification Request is D-2 and is not decided here

## RFC Documentation (Scope: protocol)

Every MUST this spec enforces takes a comment directly above the enforcing code,
naming the section and quoting the requirement, in the form
`// RFC NNNN Section X.Y: "<quoted requirement>"`. The set is:

| Code path | Section | Quoted requirement |
|-----------|---------|--------------------|
| the Notification Response builder | RFC 3748 Section 5.2 | "The peer MUST respond to a Notification Request with a Notification Response unless the EAP authentication method specification prohibits the use of Notification messages." |
| the same builder, on its Type-Data | RFC 3748 Section 5.2 | "A Response MUST be sent in reply to the Request with a Type field of 2 (Notification)." |
| the guard that keeps a Nak off the Notification path | RFC 3748 Section 5.2 | "In any case, a Nak Response MUST NOT be sent in response to a Notification Request." |
| the Nak builder | RFC 3748 Section 5.3.1 | "Where a peer receives a Request for an unacceptable authentication Type (4-253,255), or a peer lacking support for Expanded Types receives a Request for Type 254, a Nak Response (Type 3) MUST be sent." |
| the Nak Type-Data | RFC 3748 Section 5.3.1 | "The Type-Data field of the Nak Response (Type 3) MUST contain one or more octets indicating the desired authentication Type(s), one octet per Type, or the value zero (0) to indicate no proposed alternative." |
| the Nak Identifier | RFC 3748 Section 5.3.1 | "The Identifier field of a legacy Nak Response MUST match the Identifier field of the Request packet that it is sent in response to." |
| the Section 2.1 boundary | RFC 3748 Section 2.1 | "A peer MUST NOT send a Nak (legacy or expanded) in reply to a Request after an initial non-Nak Response has been sent." |
| the discard path | RFC 3748 Section 2.1 | "a peer receiving such Requests MUST treat them as invalid, and silently discard them" |
| the Type-254 arm of the Nak builder | RFC 3748 Section 5.7 | "Peers not equipped to interpret the Expanded Type MUST send a Nak as described in Section 5.3.1, and negotiate a more suitable authentication method." |

The wire format of both new packets is documented with byte offsets in
`docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`: the Notification Response is
Code, Identifier, Length of 5, Type of 2 and no Type-Data, and the legacy Nak is
Code, Identifier, Length of 6, Type of 3 and one desired-Type octet.

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

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

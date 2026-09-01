# Spec: eap-notification-and-nak

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | `plan/spec-rfcgate-6-supported-extraction-signoff.md` (its 2026-08-30 walk produced the six unclassified rfc3748 sites and the scratch sign-off this spec lands) |
| Phase | - |
| Deferral shard | - |
| Handoff | verify |
| Updated | 2026-09-01 |

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

Type 4 (MD5-Challenge) was a deliberate deviation, authorized by Thomas on
2026-08-30 and WITHDRAWN by him on 2026-09-01, when he ordered the method
implemented. The row in `plan/journal/gate-excludes-part-of-its-population.md`
carries the withdrawal beside the original. The rationale the deviation rested
on was RFC 7296 Section 2.16, at `rfc/full/rfc7296.txt` line 2958: "EAP methods
that do not establish a shared key SHOULD NOT be used, as they are subject to a
number of man-in-the-middle attacks". That reading was too strong. The sentence
is a SHOULD NOT, and the sentence after it specifies what to do when such a
method IS used, so MD5-Challenge over IKEv2 is a configuration the RFC provides
for rather than one Ze may never reach. See D-1 below.

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

### D-1: ANSWERED 2026-09-01 -- option C, implement MD5-Challenge

Thomas chose option C on 2026-09-01: implement Type 4. That WITHDRAWS the
2026-08-30 deviation, and every artifact citing it as current is corrected
rather than kept. Sites `5:2`, `5.4:1` and `5.4:2` are `mapped` to
`RFC3748-5-2`, `RFC3748-5.4-1` and `RFC3748-5.4-2`, so the closed
exclusion-kind set never had to hold a word for an authorized deviation.

Option C was priced in this spec as "full support with an unreachable entry
point", and that price rested on a claim the RFC does not make. RFC 7296
Section 2.16, at `rfc/full/rfc7296.txt` line 2958, states a SHOULD NOT rather
than a MUST NOT, and the sentence after it specifies what to do when such a
method IS used: "If EAP methods that do not generate a shared key are used, the
AUTH payloads in messages 7 and 8 MUST be generated using SK_pi and SK_pr,
respectively." RFC 3748 itself carries no prohibition on non-key-deriving
methods over IKEv2; its only mentions of IKE are a lower-layer list and two
citations. So MD5-Challenge inside IKEv2 is a specified configuration, and the
implementation is reachable rather than unwired.

One question falls out of that and is NOT answered here. `rfc/short/rfc3748.md`
carries `RFC3748-7.10-3` as `[MUST NOT] Methods that do not generate MSK ...
MUST NOT be used with IKEv2 (S7.10, RFC 7296 S2.16)`. Section 7.10 says nothing
about IKE, and Section 2.16 says SHOULD NOT, so the row states an obligation
stronger than either source. Correcting a level in `rfc/short/` is a doing-less
action under `ai/rules/rfc-compliance.md`, so it waits for Thomas. Until it is
settled, no config value selects MD5-Challenge inside an IKEv2 session: the
method and the AUTH path are built and proven, and the selection surface is not.

### D-1 (original text, superseded): how an authorized deviation is dispositioned in an extraction sign-off

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

### D-2: ANSWERED 2026-09-01 -- Ze's authenticator sends none

Thomas answered D-2 on 2026-09-01: the authenticator sends no Notification
Request. Section 5.2 states the send as a MAY, and its own text agrees that the
smaller surface is the usual one: "In most circumstances, Notification should
not be required."

Site `5.2:4` is therefore `excluded` with kind `feature-out-of-scope`, its
reason quoting the MAY and naming the scope decision, which is what
`ai/rules/rfc-compliance.md` requires of an optional feature Ze declines. The
absent feature is recorded in `rfc/short/rfc3748.md` so a later scope decision
can revisit it, and it is never recorded as a conformance gap. The mandatory
half is untouched: Ze's peer ANSWERS a Notification Request, and that is
`RFC3748-5.2-1`.

With D-1 and D-2 both answered, every site in `rfc/extraction/rfc3748.json`
carries a disposition and no relocation remains, so the sign-off this spec was
blocked on is landed.

### D-2 (original text, superseded): does Ze's authenticator ever SEND a Notification Request

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
  → Constraint: this spec also RESERVES four further ids, added 2026-09-01 so that `rfc/extraction/rfc3748.json` could sign. A `relocated-to-spec` site needs a `reserved-id` naming the row its destination owes, or the relocation points at a document rather than at a row and this spec could satisfy the gate while owing nothing. The four are `RFC3748-5-2` (Section 5, "All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254"), `RFC3748-5.2-3` (Section 5.2, "The message MUST NOT be null terminated"), `RFC3748-5.4-1` (Section 5.4, "A Response MUST be sent in reply to the Request") and `RFC3748-5.4-2` (Section 5.4, "EAP peer and EAP server implementations MUST support the MD5-Challenge mechanism"). All four are unmet today and all four wait on the same D-1: the owner authorized the Types 2, 3 and 4 deviation on 2026-08-30, and the closed `excluded-kind` set has no word for an authorized deviation, which is what D-1 asks. Writing `feature-out-of-scope` for them instead would assert a scope decision nobody has taken
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
| A-1 | "an initial non-Nak Response" in RFC 3748 Section 2.1 means the peer's first Response to an authentication METHOD Request, not the Identity Response | RFC 3748 Section 5.4: "The Response MAY be either of Type 4 (MD5-Challenge), Nak (Type 3), or Expanded Nak (Type 254)", which describes a Nak sent after an Identity Response. The literal alternative makes Section 5.3.1's MUST unreachable in every conversation | the Nak would be forbidden the moment the peer answers the Identity Request, which no implementation could satisfy | the reading is stated in the code comment above the guard, and `TestRFC3748PeerNaksBeforeItCommitsToAMethod` drives both sides of the boundary | confirmed: `PeerSession.methodCommitted` (peer.go) and `Session.methodAnswered` (eap.go) carry the reading; `TestRFC3748PeerNaksBeforeItCommitsToAMethod` drives both sides |
| A-2 | The strongSwan image built from `test/interop-ipsec/Dockerfile.strongswan` (alpine 3.21, package `strongswan`) ships the `eap-dynamic` plugin | the Alpine package is the full charon build, and the scenario needs a peer that reacts to a Nak by offering another method | the scenario falls back to the single-method form: strongSwan offers `eap-md5` only, Ze Naks, and the assertion is strongSwan's "received EAP Nak" log line plus no tunnel | `docker run --rm <image> ls /usr/lib/ipsec/plugins` before the scenario is written | **broken**: the image ships no `eap-dynamic` (`ls /usr/lib/ipsec/plugins`), so the scenario took the fallback this row named. Mistake Log and Deviations rows recorded |
| A-3 | An all-zero `PeerResult` today means "send nothing and wait", so an explicit discard outcome changes no wire behavior | `handleEAPResponse` (`internal/component/ike/engine/fsm.go`) guards the send with `if result.Response != nil` and has no else branch | the discard would need a carrier change as well as a peer change | `TestPeerDiscardLeavesTheSAAlive` drives the engine path and asserts the SA state is unchanged | confirmed: `TestPeerDiscardLeavesTheSAAlive` drives `handleEAPResponse` and asserts the SA state is unchanged |
| A-4 | Neither EAP-MSCHAPv2 nor EAP-TLS prohibits Notification messages in its own specification | RFC 3748 Section 5.2's excusing clause requires the METHOD specification to say so, and neither RFC 2759 nor RFC 5216 contains such a prohibition | the peer would owe a silent discard rather than a Response while that method runs, which is site `5.2:3` | a read of RFC 2759 and RFC 5216 for a Notification prohibition, recorded in the site's reason | confirmed: `grep -c -i notification rfc/full/rfc2759.txt` = 0 and the same over `rfc/full/rfc5216.txt` = 0, so neither specification mentions Notification at all |
| A-5 | `Session.err` is the right home for the desired Types a received Nak carried | `Session.Err()` already exists for exactly this reason: RFC 3748 Section 4.2 leaves an EAP-Failure no field to carry a reason, and `handleResponderEAP` logs it | the information would need a second channel out of the authenticator | `TestAuthenticatorRecordsTheTypesANakAskedFor` reads `Session.Err()` after a driven Nak | confirmed: `Session.nakRefused` and `Session.nakUnexpected` write `s.err`; `TestAuthenticatorRecordsTheTypesANakAskedFor` reads `Session.Err()` |

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
| `eap-nak-method-negotiation` | `test/interop-ipsec/scenarios/` | strongSwan | Ze's Type-3 Nak is a packet another implementation parses as a Nak. The desired-Type octet is NOT proven here, because the image ships no `eap-dynamic` plugin and charon never switches method; that octet is proven by `TestNakNamesTheConfiguredMethod` and by `test/ipsec/ipsec-eap-nak-unacceptable-type.ci`. Proven RED by reverting the Nak path and rebuilding `test/interop-ipsec/ze-linux` | |

## Recorded RED Proofs (AC-14)

`ai/rules/interop-and-goal-validation.md` makes the revert-and-rebuild mandatory
and not deferrable, so the observed output is recorded here rather than left in
a session transcript.

**Interop, `eap-nak-method-negotiation`.** `PeerSession.naks` was reverted to
return false on every arm, and the image was REBUILT (`ze-ipsec-interop`
sha256:7cd83f300132) so the container saw the revert; a host-side edit it never
sees proves nothing. Exit 1:

```
"error": "wait for strongswan log \"EAP/RES/NAK\" timed out before the peer became ready",
"name": "eap-nak-method-negotiation",
"passed": false
```

Restored, re-run green: `code 0 passed 1 [{'name': 'eap-nak-method-negotiation', 'passed': True}]`.

**Functional, `ipsec-eap-nak-unacceptable-type.ci`.** Same revert, exit 1:

```
TEST FAILURE: 11 ipsec-eap-nak-unacceptable-type
TYPE:    timeout
LIKELY CAUSE:
  await=stderr: daemon stderr never contained "the peer refused type 13 with a Nak asking for type 26" within 2m0s
level=WARN msg="ike: EAP packet discarded" subsystem=ike peer=peer-1 code=1 type=13 id=2
level=WARN msg="ike: responder handshake timed out, tearing down" subsystem=ike peer=initiator state=eap-in-progress
```

**Functional, `ipsec-eap-md5-challenge.ci`.** The `AuthEAPMD5` arm of
`eapMethodType` was deleted; FAIL in 2.9s with `ike: create EAP session failed
... auth mode eap-md5 is not an EAP method`. Restored, green in 4.3s.

**What the interop scenario does NOT prove.** The strongSwan image ships no
`eap-dynamic` plugin, verified by listing `/usr/lib/ipsec/plugins` in the built
image, and that is the only charon plugin that answers a Nak by offering another
method. So charon never switches and the scenario cannot prove the Nak's
desired-Type octet names the method ze runs. That octet is proven by
`TestNakNamesTheConfiguredMethod` and by
`test/ipsec/ipsec-eap-nak-unacceptable-type.ci`, where ze's own authenticator
names it.

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
| The Type-4 deviation, while it stood, was recorded in `plan/journal/gate-excludes-part-of-its-population.md`; its 2026-09-01 withdrawal is appended to the same row | a new class file named for the deviation | The class the row belongs to is the durable one: an obligation that binds Ze sat outside every checked population, first because the summary declared no row and then because the walk excluded it on circular grounds. Two rows from the same walk already sit in that file. `ai/rules/rfc-compliance.md` requires the row; it does not require a class of its own |

## Known Limitations

- Type 4 (MD5-Challenge) is implemented on both roles, by Thomas's 2026-09-01 order withdrawing the 2026-08-30 deviation. `authentication { mode eap-md5 }` selects it, default off, and adopting it logs the RFC 7296 Section 2.16 warning. `RFC3748-7.10-3` was restated at SHOULD NOT on the owner's 2026-09-01 answer, because Section 7.10 does not mention IKE and Section 2.16 states a SHOULD NOT
- Type 254 (Expanded Types) is not implemented. RFC 3748 Section 5 puts it at SHOULD, and Ze answers a Type-254 Request with the legacy Nak the RFC prescribes for a peer that lacks the support
- Ze's AUTHENTICATOR still offers exactly one method, so a received Nak ends the exchange. Offering a second method after a Nak is a feature with its own configuration surface and is not in this spec
- Ze's authenticator sends no Notification Request. That is D-2, answered by Thomas on 2026-09-01, and it is a declined option rather than a gap: RFC 3748 Section 5.2 states the send as a MAY

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

## Implementation Summary

### What Was Implemented
- **The peer answers Type 2.** `PeerSession.notificationResponse` (`internal/component/ike/eap/peer.go`) builds a Code-2/Type-2 Response with zero-octet Type-Data and the Request's Identifier, carries the displayable message out on `PeerResult.Notification` behind an explicit `PeerResult.Notified` flag, and bounds it at `notificationMax` (1015). Peer state and every method field are untouched.
- **The peer answers Type 3.** `PeerSession.naks` decides (Types 4-253, 255 and 254 by RFC 3748 Section 5.7, never the configured method, never after `methodCommitted`), and `PeerSession.nakResponse` names the one configured method in a single desired-Type octet.
- **The Section 2.1 boundary is a field, not an inference.** `PeerSession.methodCommitted` is set by `commitMethod` only when the method actually produced a Response, and `Session.methodAnswered` (`eap.go`) is its authenticator-side mirror.
- **A discard is an explicit outcome.** `peerDiscard()` returns `PeerResult{Discarded: true}`; `handleRequest` returns it for `peerStateDone`, for an Identity Requery and for any Type the three answers above do not own. `handleEAPResponse` and `startEAPExchange` (`engine/fsm.go`) log it and leave the SA in `StateEAPInProgress`.
- **The authenticator reads a Nak instead of discarding what it carried.** `Session.nakRefused` records the desired Types through `nakRefusal`/`desiredTypes` (bounded at `desiredTypeMax` = 252) into `Session.err`; `Session.nakUnexpected` discards a Nak sent after the peer committed and records why, per RFC 3748 Section 2.1's "SHOULD discard it and log the event".
- **Type 4 (MD5-Challenge) is implemented on both roles**, by the owner's 2026-09-01 withdrawal of the 2026-08-30 deviation. `md5ChallengeMethod` (`eap/eap_md5challenge.go`) is the authenticator half; `PeerSession.handleMD5ChallengeRequest` (`peer.go`) is the peer half; `md5ChallengeResponse` is the one CHAP computation both use.
- **The AUTH payload asks the method instead of reading a zero.** `eapAuthSecret` (`engine/eap_auth.go`) returns the MSK for a key-deriving method and SK_pi/SK_pr for one that derives none, gated on `Succeeded()`. `ComputeAuthFromMSK`/`VerifyAuthFromMSK` became `computeAuthFromSharedSecret`/`verifyAuthFromSharedSecret`, and `computePSKAuth`/`verifyPSKAuth` now route through them, so one formula serves all three secrets.
- **Single declarations replaced four scattered switches.** `eap.TypeDerivesKey`, `engine.eapMethodType`, `ipsec.IsEAPPasswordMode` and `ipsec.IsEAPMode` each hold one fact that `parseEAPUser`, `parseAuthConfig`, `ValidateRemoteAccess`, `eapMethodConfig`, `startEAPExchange` and `warnKeylessEAPModes` now read rather than restate.
- **The operator surface.** `enum eap-md5` in both `authentication mode` enums (`ipsec/yang/ze-ipsec-conf.yang`), `ipsec.AuthEAPMD5`, and `warnKeylessEAPModes` writing one RFC 7296 Section 2.16 warning per adoption from `runEngine` (`engine/register.go`).
- **Proof.** 9 new requirement rows in `rfc/short/rfc3748.md` with both polarities, 26 new test functions across 10 files, the `eap-nak-method-negotiation` interop scenario with `checkEAPNakMethodNegotiation` (`internal/le/interoplab/ipsec/checkers.go`), and two `.ci` functional tests.

### Bugs Found/Fixed
- **A Request of an unhandled Type killed the IKE SA.** `handleRequest` returned an error for every Type but Identity and the configured method; `handleEAPResponse` reads any non-nil `Err` as `StateDead`. One unauthenticated packet therefore ended the exchange. Covered by `TestPeerDiscardsAnIdentityRequery`, `TestPeerDiscardsARequestAfterTheExchangeCompleted` and `TestPeerDiscardLeavesTheSAAlive`.
- **A spoofed Nak ended a live authentication.** `Session.handleMethod` answered every `TypeNAK` Response with `s.failure(...)`, including one arriving after the peer had committed to the method, which RFC 3748 Section 2.1 says to discard and log. Covered by `TestAuthenticatorDiscardsAnUnexpectedNak`.
- **`sa.EAPMSK != [64]byte{}` was a zero read as an answer.** `verifyRemoteAuth` decided the EAP AUTH construction from an all-zero MSK, which is the same value for a method that derives none, one whose derivation failed, and a field nobody set. Replaced by `eapAuthSecret`. Covered by `TestEAPAuthOfNonKeyDerivingMethodUsesSKpiAndSKpr`, `TestEAPAuthOfKeyDerivingMethodStillUsesTheMSK` and `TestEAPAuthIsRefusedBeforeTheEAPExchangeSucceeds`.
- **The desired Types a Nak carried were discarded.** An operator read "authentication failed" with no way to learn which method the far end wanted. Covered by `TestAuthenticatorRecordsTheTypesANakAskedFor` and asserted end-to-end by `test/ipsec/ipsec-eap-nak-unacceptable-type.ci`.
- **An unbounded attacker-chosen string reached a log line.** `wireEAPToPacket` slices Type-Data with no cap, so a Nak's desired-Type list and a Notification message both needed one: `desiredTypeMax` (252, the RFC's own count of authentication Types) and `notificationMax` (1015, RFC 3748 Section 5.2's own budget). Covered by `TestNakDesiredTypeListIsBounded`.
- **`ai/RFC-REQUIREMENTS.md` was stale against its sources**, found by this closure's `./le rfc check` run. Fixed by `./le rfc index-update`, which rewrote it and `rfc/requirements/rfc3748.md` (proven 134 -> 140).

### Documentation Updates
- `docs/features.md` -- the "IPsec EAP Authentication" and "IKEv2 Engine" rows name MD5-Challenge and the peer's Nak/Notification behavior; the "IPsec Interop Testing" row names the new scenario. Anchors added: `<!-- source: internal/component/ike/engine/eap_auth.go -- eapMethodType, warnKeylessEAPModes -->`.
- `docs/guide/ipsec.md` -- two new sections, "EAP method negotiation" and "EAP MD5-Challenge", each with source anchors; the package table and the road-warrior paragraph name `eap-md5`.
- `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` -- the Notification Response and legacy Nak wire formats with byte offsets, and the MD5-Challenge packet shapes.
- `docs/architecture/ike/ipsec-11-interop-eap.md` -- the `eap-nak-method-negotiation` scenario, what it proves and what it does NOT prove; the `ComputeAuthFromMSK` anchor is repointed at `computeEAPAuth, eapAuthSecret`.
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` -- the discard outcome the SA survives.
- `docs/architecture/ike/ipsec-14-responder.md` and `ai/digests/ipsec-ike.md` -- the renamed producers and the two-secret AUTH rule.
- `docs/functional-tests.md` -- both new `.ci` tests are described at line 304. NOTE: the file is clean in the working tree; that edit was carried into commit `199b684f6` by a concurrent session sharing this checkout's index.
- `docs/features/rfc-status.md` -- NOT hand-edited. It is generated; `./le rfc index-update` was run.
- `docs/architecture/testing/interop.md` -- No. It does not enumerate ipsec scenario directories (`grep 'eap-mschapv2' docs/architecture/testing/interop.md` returns nothing), so a new scenario owes it no row.
- `./le doc check verify` -- exit 1, and every one of the 1,700 findings is in `../gh-pages/` or `../wiki/` and names the CLI command catalog, which commit `e691533a6` moved. Zero findings under `docs/`. Grouped by file: `grep -c` over the run gives `../gh-pages/reference/command-equivalents/index.html` 818, `../gh-pages/reference/cli/index.md` 440, `../gh-pages/reference/cli/index.html` 391, `../gh-pages/llms.txt` 391, `../wiki/command-catalog.md` 1.

### Deviations from Plan
- **A-2 broke.** The strongSwan image ships no `eap-dynamic` plugin, so the interop scenario uses the single-method fallback the assumption row named. See the Mistake Log.
- **Type 4 was implemented**, which the spec's D-1 originally priced as option C with an unreachable entry point. The owner withdrew the deviation on 2026-09-01 and chose option A for the surface, so `authentication { mode eap-md5 }` selects it, default off. `RFC3748-7.10-3` was restated at SHOULD NOT on the same answer.
- **The scope grew past the spec's Files to Modify** to cover what MD5-Challenge reached: `engine/auth.go`, `engine/eap_auth.go`, `engine/register.go`, `engine/responder_eap.go`, `ipsec/config.go`, `ipsec/types.go`, `ipsec/validate.go`, `ipsec/yang/ze-ipsec-conf.yang` and `internal/le/interoplab/ipsec/checkers.go`. Each is a site the new mode had to be answered at; none is a second mechanism.
- **`test/ipsec/ipsec-eap-md5-challenge.ci` and four MD5 test files were added** beyond the TDD plan, because the plan was written when Type 4 was excluded.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed the alpine 3.21 `strongswan` package is the full charon build and ships `eap-dynamic`, so the interop scenario could prove that Ze's Nak makes a concentrator switch method | `ls /usr/lib/ipsec/plugins` in the image built from `test/interop-ipsec/Dockerfile.strongswan` lists `eap-md5`, `eap-mschapv2`, `eap-tls`, `eap-radius`, `eap-aka`, `eap-sim` and `eap-identity`, and no `eap-dynamic`. `eap-dynamic` is the only charon plugin that answers a received Nak by offering another method | Listing the plugin directory in the built image before the scenario was written, which is the validation method the A-2 row itself named | The scenario took the fallback the row named: charon offers `eap-md5` only, Ze Naks, and the assertion is charon's own `initiating EAP_MD5 method` and `EAP/RES/NAK` lines plus no XFRM SA at either end. The desired-Type octet is proven instead by `TestNakNamesTheConfiguredMethod` and by `test/ipsec/ipsec-eap-nak-unacceptable-type.ci`. `docs/architecture/ike/ipsec-11-interop-eap.md`, the scenario's `swanctl.conf` and `checkEAPNakMethodNegotiation`'s doc comment each say what the scenario does NOT prove, so a later reader cannot mistake it |
| approach | The scenario first failed on the wrong assertion: Ze's log recorded `ike: EAP packet discarded ... code=1 type=4`, so charon HAD sent the Request, while charon's own log still ended at `parsed IKE_AUTH request 1` | charon's stderr block-buffers when it is a pipe rather than a terminal, so `docker logs` showed the buffer and not the exchange | Comparing the two daemons' logs at the moment the checker timed out | `test/interop-ipsec/scenarios/eap-nak-method-negotiation/strongswan.conf` sets `filelog { stderr { default = 1; flush_line = yes } }`, and the file records the measurement that made it necessary. `plan/journal/failing-gate-prints-no-cause.md` carries the general class |
| escalation | The generated `ai/RFC-REQUIREMENTS.md` was stale at the moment closure started, although `./le rfc index-update` had already been run once during implementation | Every discrimination record written after that run moves the index's proven counts, so the index goes stale again with each one | `./le rfc check` at closure step 1 named it | Re-ran `./le rfc index-update`. The lesson is that the index run belongs AFTER the last discrimination record, not after the last summary edit |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Implement Type 2 (Notification) on the peer | Done | `PeerSession.notificationResponse`, `internal/component/ike/eap/peer.go` | Both peer states; state and method fields untouched |
| Implement Type 3 (Nak) on the peer | Done | `PeerSession.naks` and `PeerSession.nakResponse`, `peer.go` | Types 4-253, 255 and 254; never after `methodCommitted` |
| Correct the authenticator's handling of a received Nak | Done | `Session.nakRefused` and `Session.nakUnexpected`, `internal/component/ike/eap/eap.go` | Desired Types recorded; an unexpected Nak discarded and logged |
| Land the requirement rows and both polarities for every site the two types make live | Done | `rfc/short/rfc3748.md:264, 308-316` | 9 rows; `RFC3748-2.1-3` carries no `{single-polarity}` |
| Land the rfc3748 extraction sign-off | Done | `rfc/extraction/rfc3748.json` | `./le rfc extraction-status`: rfc3748 is absent from the 126-stem `unsigned` list, so it is one of the 55 signed |
| Implement Type 4 (MD5-Challenge), per the owner's 2026-09-01 withdrawal | Done | `internal/component/ike/eap/eap_md5challenge.go`, `PeerSession.handleMD5ChallengeRequest` | Both roles; `authentication { mode eap-md5 }` selects it, default off |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPeerAnswersANotificationRequest` | Code 2, Type 2, zero-octet Type-Data, Identifier copied, 5-octet packet |
| AC-2 | Done | `TestNotificationMidMethodLeavesTheMethodUntouched` | Full MS-CHAPv2 exchange with a Notification injected between rounds |
| AC-3 | Done | `TestPeerAnswersANotificationRequest` plus `handleEAPResponse`'s `result.Notified` branch (`engine/fsm.go`) | The message leaves on `PeerResult` and is logged once, as a slog value |
| AC-4 | Done | `TestPeerNeverNaksANotificationRequest` | Zero-length and non-UTF-8 messages still draw a Type-2 Response |
| AC-5 | Done | `TestPeerNaksAnUnacceptableAuthenticationType`, `TestNakNamesTheConfiguredMethod` | Code 2, Type 3, one octet holding 26, 6-octet packet |
| AC-6 | Done | `TestPeerNaksAnExpandedTypeRequestWithALegacyNak` | Type 254 draws a legacy Nak; no Type-254 packet is ever encoded |
| AC-7 | Done | `TestPeerNaksAnUnacceptableAuthenticationType` | A Type-4 offer to an `eap-mschapv2` peer is refused by the protocol's own mechanism |
| AC-8 | Done | `TestPeerDiscardsARequestAfterTheExchangeCompleted`, `TestPeerDiscardLeavesTheSAAlive` | `Discarded` set, no Response, no Err, SA stays in `StateEAPInProgress` |
| AC-9 | Done | `TestPeerDiscardsAnIdentityRequery` | Type 1 is outside 4-253 and 255, so no Nak |
| AC-10 | Done | `TestAuthenticatorRecordsTheTypesANakAskedFor` | EAP-Failure plus `Session.Err()` naming the Types |
| AC-11 | Done | `TestNakNamesTheConfiguredMethod`, `TestNakIdentifierMatchesTheRequest` | Length 6, desired octet never 0 while a method is configured |
| AC-12 | Done | `rfc/short/rfc3748.md` | `grep -c 'RFC3748-5.2-\|RFC3748-5.3.1-\|RFC3748-5-2\|RFC3748-5.4-'` = 9; `grep 'RFC3748-2.1-3'` shows no annotation |
| AC-13 | Changed | `./le rfc check` | Exit 2, and NONE of the 15 rfc3748 findings is a row this spec wrote. They are `RFC3748-2-3`, `2.2-1`, `4.1-6..9`, `4.2-10..15`, `7.10-5..7` at `rfc/short/rfc3748.md:289-306`, all present at HEAD (`git diff -- rfc/short/rfc3748.md` changes one line) and all owed by `plan/spec-rfcgate-6-supported-extraction-signoff.md`, whose commit `f0b75088f` declares them: "Fifteen coverage violations remain, each a row declared and not yet proven." Discrimination is clean: `changed 0, escaped 0, owed 0, unresolved 0` |
| AC-14 | Done | Recorded RED Proofs section above | Interop RED under a reverted `PeerSession.naks` with the image REBUILT (`ze-ipsec-interop` sha256:7cd83f300132); functional RED for both `.ci` tests |
| AC-15 | Done | `rfc/extraction/rfc3748.json` | Every site dispositioned, no relocation left; D-1 answered, so `5:2`, `5.4:1` and `5.4:2` are `mapped` and `5.2:4` is `excluded: feature-out-of-scope` per D-2 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPeerAnswersANotificationRequest` | Done | `eap/rfc3748_notification_test.go` | |
| `TestPeerNeverNaksANotificationRequest` | Done | `eap/rfc3748_notification_test.go` | |
| `TestNotificationMidMethodLeavesTheMethodUntouched` | Done | `eap/rfc3748_notification_test.go` | |
| `TestNotificationRequestsCountAgainstTheRoundCap` | Done | `eap/rfc3748_notification_test.go` | |
| `TestPeerNaksAnUnacceptableAuthenticationType` | Done | `eap/rfc3748_nak_test.go` | |
| `TestPeerNaksAnExpandedTypeRequestWithALegacyNak` | Done | `eap/rfc3748_nak_test.go` | |
| `TestNakIdentifierMatchesTheRequest` | Done | `eap/rfc3748_nak_test.go` | |
| `TestPeerDoesNotNakAMethodError` | Done | `eap/rfc3748_nak_test.go` | |
| `TestRFC3748PeerNaksBeforeItCommitsToAMethod` | Done | `eap/rfc3748_nak_test.go` | Carries the `RFC3748-2.1-3` positive and negative tags |
| `TestPeerDiscardsAnIdentityRequery` | Done | `eap/rfc3748_nak_test.go` | |
| `TestAuthenticatorRecordsTheTypesANakAskedFor` | Done | `eap/rfc3748_nak_test.go` | |
| `TestRFC3748ResponseTypeMatchesRequest` (corrected) | Done | `eap/rfc3748_walk_test.go:213` | The `RFC3748-4.1-5` negative arm now asserts a Type-3 Nak carrying `{TypeMSCHAPv2}` |
| `TestEngineAnswersANotificationRequest` | Done | `engine/eap_wiring_test.go` | |
| `TestEngineSendsANakForAnUnacceptableType` | Done | `engine/eap_wiring_test.go` | |
| `TestPeerDiscardLeavesTheSAAlive` | Done | `engine/eap_wiring_test.go` | |
| `ipsec-eap-nak-unacceptable-type` | Done | `test/ipsec/ipsec-eap-nak-unacceptable-type.ci` | |
| `eap-nak-method-negotiation` | Changed | `test/interop-ipsec/scenarios/eap-nak-method-negotiation/` | Single-method fallback; see the A-2 Mistake Log row |
| 11 further tests, added beyond the plan | Changed | `eap/rfc3748_md5challenge_test.go`, `eap/method_set_test.go`, `eap/spoofed_packet_test.go`, `engine/rfc3748_ikev2_method_selection_test.go`, `engine/rfc7296_eap_nonkeying_auth_test.go`, `engine/eap_password_mode_test.go`, `ipsec/eap_password_mode_test.go`, `test/ipsec/ipsec-eap-md5-challenge.ci` | The plan was written while Type 4 was excluded |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/eap/peer.go` | Done | |
| `internal/component/ike/eap/eap.go` | Done | |
| `internal/component/ike/engine/fsm.go` | Done | |
| `internal/component/ike/eap/rfc3748_test.go` | Done | `TestRFC3748PeerNeverSendsNAK` deleted, tag moved; `TestRFC3748IKEv2RequiresKeyDerivingMethod` deleted, tags moved to the engine |
| `internal/component/ike/eap/rfc3748_walk_test.go` | Done | |
| `rfc/short/rfc3748.md` | Done | The nine rows are at HEAD; the working tree changes one `not-applicable` reason to name MD5-Challenge |
| `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` | Done | |
| `docs/architecture/ike/ipsec-11-interop-eap.md` | Done | |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Done | |
| `docs/architecture/core-design.md` | Changed | Untouched, as the row predicted for any answer but option A to D-1's original framing |
| `docs/features/rfc-status.md` | Done | Regenerated by `./le rfc index-update`, never hand-edited |
| `docs/features.md` | Done | |
| `plan/journal/gate-excludes-part-of-its-population.md` | Done | Carries the deviation and its 2026-09-01 withdrawal |
| `internal/component/ike/eap/rfc3748_notification_test.go` | Done | Created |
| `internal/component/ike/eap/rfc3748_nak_test.go` | Done | Created |
| `internal/component/ike/engine/eap_wiring_test.go` | Done | Created |
| `test/ipsec/ipsec-eap-nak-unacceptable-type.ci` | Done | Created |
| `test/interop-ipsec/scenarios/eap-nak-method-negotiation/swanctl.conf` | Done | Created |
| `test/interop-ipsec/scenarios/eap-nak-method-negotiation/ze.conf` | Done | Created |
| `test/interop-ipsec/scenarios/eap-nak-method-negotiation/strongswan.conf` | Changed | Created, beyond the plan: charon block-buffers stderr through a pipe |
| `rfc/extraction/rfc3748.json` | Done | Landed |
| 9 further files | Changed | `engine/auth.go`, `engine/eap_auth.go`, `engine/register.go`, `engine/responder_eap.go`, `ipsec/config.go`, `ipsec/types.go`, `ipsec/validate.go`, `ipsec/yang/ze-ipsec-conf.yang`, `internal/le/interoplab/ipsec/checkers.go` -- the sites MD5-Challenge reached |

### Audit Summary
- **Total items:** 62 (6 requirements, 15 ACs, 18 tests, 23 files)
- **Done:** 55
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 7 (AC-13, the interop scenario, the 11 extra tests, `docs/architecture/core-design.md`, `strongswan.conf`, the 9 extra files, the extra `.ci`) -- each recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An authenticator that offers a method Ze does not run gets a Type-3 Response naming the method Ze runs, instead of an error and a dead IKE SA | interop | `eap-nak-method-negotiation` under `./le integration interop-ipsec`: `code 0 passed 1 [{'name': 'eap-nak-method-negotiation', 'passed': True}]`. The assertion is strongSwan's own `initiating EAP_MD5 method` then `EAP/RES/NAK`, so charon decoded Ze's octets as a Nak. RED proven by reverting `PeerSession.naks` and REBUILDING the image (`ze-ipsec-interop` sha256:7cd83f300132): `wait for strongswan log "EAP/RES/NAK" timed out before the peer became ready`, exit 1 |
| An operator can see which method the far end asked for | functional | `test/ipsec/ipsec-eap-nak-unacceptable-type.ci`: two daemons, one configured `mode eap-tls` as authenticator and one `mode eap-mschapv2` as peer, asserting `the peer refused type 13 with a Nak asking for type 26` and `EAP authentication failed` on stderr. RED under the same revert: `await=stderr ... never contained` within 2m0s |
| Ze answers a Notification Request instead of erroring | functional (unit over the engine entry point) | `TestEngineAnswersANotificationRequest` (`engine/eap_wiring_test.go`) drives `handleEAPResponse`, not `handleRequest`, so the Response reaching the wire is what is asserted |
| One unauthenticated packet can no longer end an EAP exchange | data correctness | Three tests, each over the real entry point: `TestPeerDiscardLeavesTheSAAlive` (SA state unchanged after a discard), `TestAuthenticatorDiscardsAnUnexpectedNak` (a Nak after commitment is discarded and recorded), `TestPeerDiscardsARequestAfterTheExchangeCompleted` |
| Type 4 is supported on both roles and reachable by an operator | functional | `test/ipsec/ipsec-eap-md5-challenge.ci`: both daemons carry `authentication { mode eap-md5 }`, the tunnel establishes (`expect=exit:code=0`, `engine-steps: all steps passed`), and the warning is asserted verbatim: `this peer runs an EAP method that establishes no shared key` and `EAP methods that do not establish a shared key SHOULD NOT be used`. RED proven by deleting the `AuthEAPMD5` arm of `eapMethodType`: FAIL in 2.9s with `ike: create EAP session failed ... auth mode eap-md5 is not an EAP method` |
| A method that derives no key still authenticates its IKEv2 AUTH correctly | data correctness | `TestEAPAuthOfNonKeyDerivingMethodUsesSKpiAndSKpr` drives a real MD5-Challenge exchange and asserts the AUTH is the SK_pi/SK_pr construction; `TestEAPAuthOfKeyDerivingMethodStillUsesTheMSK` is the counter-case; `TestEAPAuthIsRefusedBeforeTheEAPExchangeSucceeds` pins the ordering guard RFC 7296 Section 2.16 requires |
| Every RFC 3748 site the two types make live carries a row and both polarities | RFC gate | 9 rows at `rfc/short/rfc3748.md:264, 308-316`, none reported by `./le rfc check`; 36 discrimination records verify across rfc3748 (34) and rfc7296 (2) with `discrimination-changed 0, escaped 0, owed 0, unresolved 0` |
| The rfc3748 extraction sign-off lands | RFC gate | `./le rfc extraction-status`: rfc3748 is absent from the 126-stem `unsigned` list, so it is one of the 55 signed. `rfc/extraction/rfc3748.json` is 67,948 bytes under `rfc/extraction/`, not scratch |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No shard. The spec metadata declares `Deferral shard \| -`, and `ls plan/deferrals/eap-notification-and-nak.md` finds no file | done | Nothing to resolve, and nothing to `remove`. No foreign shard was emptied by this work: this closure set no row in any other spec's shard to a terminal status |
| D-1, carried in the spec's own Owner Decisions rather than in a shard | done | Answered 2026-09-01: implement Type 4, and make it operator-selectable (option A for the surface). Implemented; the 2026-08-30 deviation is withdrawn in `plan/journal/gate-excludes-part-of-its-population.md` |
| D-2, same | done | Answered 2026-09-01: the authenticator sends no Notification Request. Site `5.2:4` is `excluded` with kind `feature-out-of-scope`; the absent feature is recorded in Known Limitations, never as a conformance gap |
| The four ids `plan/spec-rfcgate-6-supported-extraction-signoff.md` relocated to this spec | done | `RFC3748-5-2`, `RFC3748-5.2-3`, `RFC3748-5.4-1` and `RFC3748-5.4-2`. Three are now rows in `rfc/short/rfc3748.md` (lines 308, 315, 316) with tests; `RFC3748-5.2-3` is covered by `RFC3748-5.2-1`'s Type-Data assertion in `TestPeerAnswersANotificationRequest` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/eap-notification-and-nak-16e17751-e30b-42b6-a39a-c8a12d282c82.md` |
| `review check` | clean. `./le spec session review check` returns `review_gate: OK (39 code files, clean, hashes match ...)`, exit 0. `./le commit review-check spec eap-notification-and-nak` returns `clean true`, `verdict clean`, exit 0, over all 39 code files |
| Rounds | 3. Round 1 and round 2 ran during implementation (one full independent 3-lens `/ze-review`, every BLOCKER and ISSUE fixed). Round 3 is this closure's fresh pass over the COMPLETE diff INCLUDING those fixes, and it found one ISSUE, fixed here |
| Outstanding before the commit | TWO preconditions, neither a finding about the diff and neither this session's to satisfy. (1) `test/rfc-changed.md` carries no owner row for five changed RFC-tagged units, enumerated under Pre-Commit Verification; `./le commit create` refuses commit A without them and only Thomas writes them. (2) `plan/spec-rfcgate-6-supported-extraction-signoff.md:866` cites this spec by full path and must be restated to the bare stem on commit A; that file is another session's in-flight work. The review verdict is CLEAN because both are facts about approval and about other sessions' files, not about the reviewed code |
| Reviewer lenses used | Round 3: (1) logic and wiring -- every product hunk read at the producing function, the AUTH role pairing checked in both directions, the two Section 2.1 boundary fields checked for asymmetry; (2) security and edge cases -- unauthenticated input bounds, constant-time comparison, zero-as-answer, panic reachability from a socket; (3) gate and record integrity -- `./le repository check`, `./le commit audit`, `./le rfc check`, `./le doc check verify`, `gofmt`, `go vet`, and every claim in this spec's own closure prose re-verified against source |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `ai/RFC-REQUIREMENTS.md` is stale against its sources, so the generated index disagrees with the summaries it derives from. Root cause: the index carries the discrimination proven/backlog counts (`Proven: 134` vs the tree's 140), and every record written after the last `./le rfc index-update` moves them, so the run made during implementation went stale again with each later record | `ai/RFC-REQUIREMENTS.md`, generated by `./le rfc index-update` | `[source]` Re-ran `./le rfc index-update`, which rewrote `ai/RFC-REQUIREMENTS.md` and `rfc/requirements/rfc3748.md` (`Proven: 134 (mutant 3, revert 131)` -> `Proven: 140 (mutant 3, revert 137)`). `[workaround]` rejected: hand-editing a generated file is destroyed at the next run with nothing saying so |
| 2 | NOTE | `md5ChallengeMethod.Process` clears the credential with `m.secret = ""`, which drops the reference but leaves the string's backing bytes in the heap, and `md5ChallengeResponse` leaves a copy of the secret in its 128-byte stack buffer | `internal/component/ike/eap/eap_md5challenge.go` | Not fixed, and recorded as a NOTE. Go strings are immutable, so there is no in-place clear; a `[]byte` credential through `MethodConfig` would be the fix, and it is a change to a field EAP-MSCHAPv2 and the config parser share. NOTEs do not block |
| 3 | NOTE | `./le repository check` reports one issue: `ParseEncryptionAlgo` in `internal/component/ike/ipsec/types.go` has no cross-package non-test caller. The symbol is at HEAD and untouched by this diff, whose only hunk in that file adds `AuthEAPMD5` to the `AuthMode` enum | `internal/component/ike/ipsec/types.go`, `ParseEncryptionAlgo` | Not fixed: a defect this work did not produce and does not depend on (`ai/rules/principles.md`). No journal row added -- `plan/journal/unwired-feature.md` already carries 77 rows of this class |
| 4 | NOTE | `golangci-lint` reports `eap/rfc5216_msk_label_test.go:30` (unlambda) and `eap/rfc2759_failure_packet_test.go:26` (unparam). Neither line is in this diff: the first file is unmodified, and the second's only hunk is a `SplitSeq`/`CutPrefix` modernization at line 85 belonging to a concurrent session | `internal/component/ike/eap/` | Not fixed. Another session's work in a shared checkout |
| 5 | NOTE | Commit B removes this spec, and one sibling spec cites it by full path: `plan/spec-rfcgate-6-supported-extraction-signoff.md:866` reads "7 belong to `plan/spec-eap-notification-and-nak.md`". `speccitation.Scan` matches the full path, and `find_dangling` resolves with `(repo / ref).is_file()`, so the citation reads GREEN today and goes red the moment the file is gone (`./le spec citation` is currently `OK (211 specs, 51 baselined dangling, 10 line-token WARN)`) | `plan/spec-rfcgate-6-supported-extraction-signoff.md:866` | **Not fixed, and deliberately.** The correct edit is the bare-stem restatement `spec-eap-notification-and-nak`, and it must ride on commit A. That file is another session's in-flight work, uncommitted in this shared checkout, so editing it from here would carry their diff into this commit (`ai/rules/principles.md`). Named for the main thread. `plan/deferrals/rfcgate-6-supported-extraction-signoff.md:22` and `plan/journal/gate-excludes-part-of-its-population.md:5` also cite the path, and both are outside the gate, which reads `plan/spec-*.md` only. `internal/component/ike/engine/eap_wiring_test.go:203` cites it in a comment; it is outside the gate too, and the comment stays true as a historical reference |
| 6 | NOTE | `docs/features.md` carries a row this work did not write: "Published RFC conformance ledger", belonging to `plan/spec-publish-the-rfc-requirement-ledger.md`. Commit A must declare the file, so that row rides along | `docs/features.md` | Not fixed. Named so the main thread can decide whether to wait for that session or carry the row |

## Pre-Commit Verification

### Verification run

| Command | Result |
|---------|--------|
| `./le verify worktree` | **Not applicable to this diff, and recorded rather than skipped.** `le verify worktree [commit <revision>]` runs the population against a fixed COMMIT in a fresh detached worktree, and this work is uncommitted, so a run defaulting to HEAD would verify a tree that does not carry it. It is owed on commit A, and the commit script runs it. `./le verify status check` reads `STALE: no status file (never verified)` for this checkout |
| `./le verify current mode full` | The in-place population over the working tree, which DOES carry the diff. Its first stage, `verify lint/run`, completed: 78 findings repo-wide across 20 build flavors, of which exactly two are under `internal/component/ike/` and neither is a line this diff wrote (`eap/rfc5216_msk_label_test.go:30`, unmodified here; `eap/rfc2759_failure_packet_test.go:26`, whose only hunk here is a concurrent session's `SplitSeq`/`CutPrefix` edit at line 85). The remaining 39 stages were still running when this closure ended, and the run is left for the commit |
| `go test -count=1 ./internal/component/ike/...` | exit 0. All 10 packages `ok`: `eap` 2.181s, `engine` 30.635s, `ipsec` 0.023s, plus cmd, crypto, dataplane, transport, wire, yang |
| `go vet ./internal/component/ike/...` | exit 0 |
| `gofmt -l internal/component/ike/` | no output |
| `./le repository check` | exit 1, one issue: `internal/component/ike/ipsec/types.go:55 ParseEncryptionAlgo has no cross-package non-test caller`. At HEAD and untouched (this diff edits the same file from line 157) |
| `./le rfc check` | exit 2, 15 rfc3748 findings, all at `rfc/short/rfc3748.md:289-306`, all at HEAD, all owed by `plan/spec-rfcgate-6-supported-extraction-signoff.md`. Discrimination clean: `changed 0, escaped 0, owed 0, unresolved 0` |
| `./le rfc extraction-status` | exit 0. rfc3748 absent from the 126-stem `unsigned` list, so it is one of the 55 signed |
| `./le doc check verify` | exit 1. Zero findings under `docs/`; all 2,047 are in the sibling `../gh-pages/` and `../wiki/` checkouts and name the CLI command catalog commit `e691533a6` moved |
| `./le commit audit` | exit 1, 12 weakened findings. See the note under Files Exist |

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/eap/rfc3748_notification_test.go` | Yes | `ls` lists it; 357 lines |
| `internal/component/ike/eap/rfc3748_nak_test.go` | Yes | `ls` lists it; 494 lines |
| `internal/component/ike/engine/eap_wiring_test.go` | Yes | `ls` lists it; 259 lines |
| `internal/component/ike/eap/eap_md5challenge.go` | Yes | `ls` lists it; 246 lines |
| `test/ipsec/ipsec-eap-nak-unacceptable-type.ci` | Yes | `-rw-rw-r-- 7255 Sep 1 12:42` |
| `test/ipsec/ipsec-eap-md5-challenge.ci` | Yes | `-rw-rw-r-- 8558 Sep 1 16:46` |
| `test/interop-ipsec/scenarios/eap-nak-method-negotiation/` | Yes | `ls` lists `strongswan.conf`, `swanctl.conf`, `ze.conf` |
| `rfc/extraction/rfc3748.json` | Yes | `-rw-r--r-- 67948 Sep 1 22:49`, under `rfc/extraction/` and not scratch |

### Owner approval still owed before commit A (BLOCKING for `./le commit create`)

`./le commit audit` reports 12 weakened findings over this diff. `test/rfc-changed.md`
carries four rows, all dated 2026-09-01 and all naming Thomas's approval. The rows
resolve findings by test NAME, and `./le commit create` is the only run that resolves
them against the DECLARED file set, so the exact remainder is known there. What this
closure can name is the set of changed RFC-tagged units for which NO row exists:

| File | Tagged unit | Why it changed | Row exists |
|------|-------------|----------------|------------|
| `internal/component/ike/eap/rfc3748_test.go` | `TestRFC3748PeerNeverSendsNAK` (deleted) | Its own words asserted the non-conformant behavior: the peer "errors (never NAKs) on an unexpected type" and "must error instead". RFC 3748 Section 5.3.1 requires the Nak. Tags `RFC3748-2.1-3 positive` and `RFC3748-7.10-3`, both re-homed: `RFC3748-2.1-3` to `TestRFC3748PeerNaksBeforeItCommitsToAMethod` (`eap/rfc3748_nak_test.go`) with BOTH polarities, `RFC3748-7.10-3` to `engine/rfc3748_ikev2_method_selection_test.go` | **No** |
| `internal/component/ike/engine/rfc7296_eap_auth_producer_test.go` | file | Rename only: `ComputeAuthFromMSK`/`VerifyAuthFromMSK` -> `computeAuthFromSharedSecret`/`verifyAuthFromSharedSecret`, plus `[64]byte` -> `[:]` at the call. No assertion moved | **No** |
| `internal/component/ike/engine/rfc7296_wp2_test.go` | file | Same rename, same reason. No assertion moved | **No** |
| `internal/component/ike/engine/responder_test.go` | file | Rename only: `NewEAPSession` -> `newEAPSession` in one comment and one `t.Fatal` string. No assertion moved | **No** |
| `internal/component/ike/eap/rfc3748_identifier_test.go` | file | Interface conformance only: `doneMethod` gains `DerivesKey()`, which `Method` now requires. No assertion moved | **No** |
| `internal/component/ike/eap/rfc2759_failure_packet_test.go` | file | Not this work: a concurrent session's `SplitSeq`/`CutPrefix` modernization | Out of scope; drops out once commit A declares only this work's files |

No row was written for any of them. `test/rfc-changed.md` says a row is the OWNER's
approval, so writing one on this session's own initiative would be a forgery by that
file's own words (`ai/rules/rfc-compliance.md`).

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-4 | The peer answers Type 2 and never Naks one | `go test -run 'RFC3748(Notification\|Nak\|Peer)' ./internal/component/ike/eap/` -> `ok ... 0.007s`, 9 PASS lines |
| AC-5..AC-7, AC-11 | The peer Naks 4-253, 255 and 254, naming the configured method | Same run; `TestRFC3748PeerNaksBeforeItCommitsToAMethod` PASS |
| AC-8, AC-9 | A Request the peer does not own is discarded, not errored | `go test -run 'TestEngine(Answers\|Sends)\|TestPeerDiscardLeavesTheSAAlive' ./internal/component/ike/engine/` -> 3 PASS, `ok ... 0.055s` |
| AC-10 | The authenticator names the Types a Nak asked for | `TestAuthenticatorRecordsTheTypesANakAskedFor` in the eap run above |
| AC-12 | 9 rows, no stale annotation | `grep -c 'RFC3748-5.2-\|RFC3748-5.3.1-\|RFC3748-5-2\|RFC3748-5.4-' rfc/short/rfc3748.md` = 9; `grep -n 'RFC3748-2.1-3' rfc/short/rfc3748.md` -> line 264, no `{single-polarity` |
| AC-13 | `./le rfc check` reports nothing against this spec's rows | Exit 2; all 15 rfc3748 findings are at lines 289-306 and belong to `spec-rfcgate-6-supported-extraction-signoff`, present at HEAD |
| AC-14 | Both RED proofs observed | Recorded verbatim in the Recorded RED Proofs section, with the rebuilt image digest |
| AC-15 | The sign-off is landed | `./le rfc extraction-status`: rfc3748 absent from `unsigned` (126 stems), so it is one of the 55 `signed` |
| Whole package | Nothing regressed | `go test -count=1 ./internal/component/ike/...` -> exit 0, all 10 packages `ok` (`eap` 2.181s, `engine` 30.635s, `ipsec` 0.023s) |
| Renames left no dangling caller | `ComputeAuthFromMSK`, `VerifyAuthFromMSK`, `NewEAPSession` have no remaining reference | `grep -rn 'ComputeAuthFromMSK\|VerifyAuthFromMSK\|NewEAPSession' --include='*.go' .` -> no hits in product or test code |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| IKE_AUTH carrying an EAP Request of Type 4 while the SA is in `StateEAPInProgress` | `test/ipsec/ipsec-eap-nak-unacceptable-type.ci` | Yes. Read the file: it starts two daemons, one `mode eap-tls` and one `mode eap-mschapv2` (lines 80, 133), and asserts `the peer refused type 13 with a Nak asking for type 26` at lines 152-154, which is Ze's own authenticator reading Ze's own Nak. Also `TestEngineSendsANakForAnUnacceptableType`, which drives `handleEAPResponse` |
| IKE_AUTH carrying an EAP Request of Type 2 | none (unit over the engine entry point) | Yes. `TestEngineAnswersANotificationRequest` (`engine/eap_wiring_test.go`) calls `handleEAPResponse`, not `handleRequest`, so the send path is exercised. No `.ci` exists: no operator surface changes for a Notification beyond the log line |
| IKE_AUTH carrying an unexpected Type after the method began | none (unit over the engine entry point) | Yes. `TestPeerDiscardLeavesTheSAAlive` asserts the SA state is unchanged after `handleEAPResponse` returns |
| strongSwan offering a method Ze does not run | `test/interop-ipsec/scenarios/eap-nak-method-negotiation/` | Yes. Read the checker: `checkEAPNakMethodNegotiation` (`internal/le/interoplab/ipsec/checkers.go`) waits for charon's `initiating EAP_MD5 method` then `EAP/RES/NAK`, then requires zero XFRM SAs at both ends, in that order, so a scenario that never reached EAP fails rather than passing on an empty kernel |
| An operator configures `authentication { mode eap-md5 }` | `test/ipsec/ipsec-eap-md5-challenge.ci` | Yes. Read the file: both daemons carry `mode eap-md5` (lines 107, 195), the tunnel establishes (`expect=exit:code=0`, `engine-steps: all steps passed`), and the adoption warning is asserted verbatim (lines 221-222) |

### Security Review (step 2)

| Check (from the spec's Security Review Checklist) | Finding |
|---|---|
| Input validation -- an unauthenticated Notification message reaching a log line | Passes. `handleEAPResponse` (`engine/fsm.go`) passes it as a slog VALUE (`"message", result.Notification`), never into a format string, a path or a command. `notificationResponse` truncates at `notificationMax` = 1015, which is RFC 3748 Section 5.2's own budget |
| Resource exhaustion -- Notification Requests are free to repeat | Passes. `PeerSession.Process` counts every Request against `maxEAPRounds` BEFORE dispatch, so a Notification consumes a round. `TestNotificationRequestsCountAgainstTheRoundCap` pins it. The log line is bounded by the same 1015-octet cut |
| The same question, asked of the Nak the AUTHENTICATOR receives | Found and closed in this diff. `wireEAPToPacket` slices Type-Data out of the whole EAP payload with no cap, so one IKE_AUTH could carry tens of thousands of octets into `Session.err` and from there into an operator's log. `desiredTypeMax` = 252 bounds it, which is the RFC's own count of authentication Types (4 through 255), and the render says `(truncated)` when it stopped short. `TestNakDesiredTypeListIsBounded` pins it |
| Error leakage -- the Nak names the configured method | Accepted, as the checklist predicted. It is the negotiation RFC 3748 Section 5.3.1 requires and it discloses nothing an authenticator does not learn from a successful exchange |
| Authorization that could fail open -- can the discard end the exchange as a success | No. `peerDiscard()` returns `PeerResult{Discarded: true}` with `Done` false, no MSK and no Response, and `handleEAPResponse` has no branch that reads a discard as completion. `TestPeerDiscardLeavesTheSAAlive` asserts the SA stays in `StateEAPInProgress` |
| Downgrade -- can a Nak steer Ze onto a keyless method | No. `nakResponse` writes `ps.method`, the ONE configured method, so the Nak names what the operator chose and never a Type an attacker picked. `naks` returns false for `ps.method`, so a Request for the configured method is never refused |
| Cryptographic misuse (generic) | `md5ChallengeMethod.Process` compares with `subtle.ConstantTimeCompare`, and `constantTimeEqualAuth` (`engine/eap_auth.go`) now calls the same primitive instead of a hand-rolled loop. `md5ChallengeMethod.Start` draws its challenge from `crypto/rand`, not a counter, which RFC 1994 Section 2.3 requires. MD5 is used only where RFC 3748 Section 5.4 prescribes it, with a `//nolint:gosec` naming the section |
| Privilege escalation / fail-open in the AUTH path | Closed in this diff, and it is the most consequential security change here. `eapAuthSecret` refuses until `Succeeded()`, so a peer cannot send its AUTH on the first EAP round and skip authenticating: SK_pi and SK_pr are derived from SKEYSEED, which anybody who completed IKE_SA_INIT holds. A key-deriving method was covered by the MSK being zero until success; a keyless method has no such accident, so the guard is written. `TestEAPAuthIsRefusedBeforeTheEAPExchangeSucceeds` pins it |
| Buffer overflow / out-of-bounds | Every length in `handleMD5ChallengeRequest` and `md5ChallengeMethod.Process` is checked against the Type-Data actually present before it indexes: empty Type-Data, `Value-Size` 0, and `len(td) < 1+valueSize` each return an error. `TestMD5ChallengeRefusesMalformedTypeData` drives them |
| Panic reachable from a socket (`docs/contributing/ze-go-style.md`, style pass question 1) | None. Three panics exist in the changed packages: `eap/eap.go:641` (a method setting `FinalRequest` with no `Err`, reachable only from Ze's own method code and never from a peer's bytes) and `engine/auth.go:798` and `:807` (DER-encoding a compile-time constant). All three are at HEAD and untouched by this diff |
| Information leakage | `md5ChallengeMethod.Process` drops the credential (`m.secret = ""`) on both arms, before returning either verdict. See NOTE 2 in the Review Gate for what that does and does not clear |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | "An initial non-Nak Response" is the peer's first Response to a METHOD Request. The reading is stated above `PeerSession.methodCommitted` (`peer.go`) and mirrored above `Session.methodAnswered` (`eap.go`), and `TestRFC3748PeerNaksBeforeItCommitsToAMethod` drives both sides of the boundary: a Nak before the commitment, a discard after it. The literal alternative would make RFC 3748 Section 5.3.1's MUST unreachable, and Section 5.4's "The Response MAY be either of Type 4 (MD5-Challenge), Nak (Type 3), or Expanded Nak (Type 254)" describes exactly the Nak that follows an Identity Response |
| A-2 | **broken** | The strongSwan image ships NO `eap-dynamic` plugin. `ls /usr/lib/ipsec/plugins` in the image built from `test/interop-ipsec/Dockerfile.strongswan` lists `eap-md5`, `eap-mschapv2`, `eap-tls`, `eap-radius`, `eap-aka`, `eap-sim` and `eap-identity`. The A-2 row's own fallback was taken: strongSwan offers `eap-md5` only, Ze Naks, and the assertion is charon's log plus no tunnel. Mistake Log row and Deviations entry recorded |
| A-3 | confirmed | `handleEAPResponse` (`engine/fsm.go`) guards the send with `if result.Response != nil` and has no else branch, so an all-zero `PeerResult` sent nothing and waited. `TestPeerDiscardLeavesTheSAAlive` drives the engine path and asserts the SA state is unchanged, so the explicit `Discarded` outcome changed no wire behavior; what changed is that a caller must now branch on it |
| A-4 | confirmed | Neither specification prohibits Notification messages. `grep -c -i 'notification' rfc/full/rfc2759.txt` = 0 and `grep -c -i 'notification' rfc/full/rfc5216.txt` = 0: the word does not appear in either document. Recorded above `PeerSession.notificationResponse`, which names RFC 3748 Section 5.4, RFC 2759 and RFC 5216 and concludes all three methods owe the Response |
| A-5 | confirmed | `Session.err` carries it. `Session.nakRefused` writes `nakRefusal(...)` and `Session.nakUnexpected` writes its own sentence, both into `s.err`, which `Session.Err()` returns and `handleResponderEAP` logs. `TestAuthenticatorRecordsTheTypesANakAskedFor` reads `Session.Err()` after a driven Nak. `Err`'s doc comment was widened to say that a non-nil value no longer means the exchange ended, so a caller reads `Succeeded()` for the outcome |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1. New user-facing feature -- Yes | `docs/features.md`, "IPsec EAP Authentication" row: MD5-Challenge type 4, the SK_pi/SK_pr AUTH, the adoption warning, and the peer's Nak and Notification behavior. Checked against `eapMethodType`, `warnKeylessEAPModes` and `PeerSession.nakResponse` | Yes |
| 2. Config syntax changed -- Yes (the checklist said No; the answer changed with D-1) | `enum eap-md5` added to BOTH `authentication mode` enumerations in `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` (the `remote-access` one at line 254 and the `site-to-site` one at line 566), each with a description quoting RFC 7296 Section 2.16. `docs/guide/ipsec.md` documents it. Verified against the YANG, not against the spec | Yes |
| 3. CLI command -- No | No `internal/component/ike/cmd/` file is in the diff | Yes |
| 4. API/RPC -- No | No RPC type in the diff | Yes |
| 6. User guide page -- Yes | `docs/guide/ipsec.md` gains "EAP method negotiation" and "EAP MD5-Challenge", both with `<!-- source: -->` anchors naming `handleRequest`, `nakResponse`, `notificationResponse`, `md5ChallengeMethod`, `eapMethodType`, `warnKeylessEAPModes`, `eapAuthSecret` | Yes |
| 7. Wire format -- Yes | `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` +122 lines carrying the Notification Response and legacy Nak shapes with byte offsets, plus the CHAP Challenge/Response body | Yes |
| 9. RFC behavior -- Yes | `rfc/short/rfc3748.md` carries the 9 rows; `docs/features/rfc-status.md` is GENERATED and was regenerated by `./le rfc index-update`, never hand-edited | Yes |
| 10. Test infrastructure -- Yes | `docs/functional-tests.md:304` describes both new `.ci` tests. The file is clean in the working tree because a concurrent session's commit `199b684f6` carried that edit. `docs/architecture/testing/interop.md` owes nothing: `grep 'eap-mschapv2'` over it returns no hit, so it does not enumerate scenario directories | Yes |
| 11. Daemon comparison -- No | `docs/comparison.md` compares feature presence and already lists EAP; the support level did not change | Yes |
| 12. Internal architecture -- Yes | `docs/architecture/ike/ipsec-7-ikev2-engine.md` (+32, the discard outcome), `ipsec-11-interop-eap.md` (the scenario and what it does not prove), `ipsec-14-responder.md`, `ai/digests/ipsec-ike.md` | Yes |
| 16. Changed files behind existing anchors -- Yes | `peer.go` -> `ipsec-11-interop-eap.md`, `eap.go` -> `ipsec-9-ikev2-eap-nat.md`, `fsm.go` -> `ipsec-7-ikev2-engine.md`, `eap_auth.go` -> `ipsec-11-interop-eap.md` (repointed from `ComputeAuthFromMSK` to `computeEAPAuth, eapAuthSecret`), `auth.go` -> `ipsec-14-responder.md`. All five edited | Yes |
| 17. Existing examples still valid -- Yes | `docs/guide/ipsec.md`'s existing EAP examples are unchanged and stay valid: no existing leaf changed, one enum value was added | Yes |
| Doctor check needed -- No | The change adds no file path, socket, port, module, binary or certificate. `eap-md5` reuses the `pre-shared-secret` leaf and the same PKI refs every EAP mode already needs (`ValidatePKIRefs`, `ipsec/validate.go`) | Yes |
| `./le doc check verify` | Exit 1. All findings are in `../gh-pages/` (2,046) and `../wiki/` (1), every one naming the CLI command catalog that commit `e691533a6` moved. Zero findings under `docs/` | Yes |

## Core Insight

**One refusal path had one shape for four different situations, and the RFC gives each a different answer.** `PeerSession.handleRequest` returned an error for an unacceptable method, an out-of-order Request, an Identity Requery and a malformed method payload alike, and the carrier read every error as `StateDead`. So the single most consequential change here is not the Nak or the Notification: it is that the peer now has four outcomes where it had one, and three of them are decided by the Type BEFORE any method sees the packet. The same shape appears again one layer down. `sa.EAPMSK != [64]byte{}` was one test standing in for two questions -- "did the exchange succeed" and "does this method derive a key" -- and an all-zero MSK answers neither. Both defects are the same mistake: a value that happened to correlate with the answer was read AS the answer, and the correlation held only while the population was small. Widening the population (a third EAP method, a fourth peer outcome) is what exposed both.

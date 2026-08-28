# Spec: ipsec-auth-piggyback

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; not started; the spec already says outstanding work needs a row here. Create `plan/deferrals/ipsec-auth-piggyback.md` on the first deferral) |
| Updated | 2026-08-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Let the IKE SA survive a Child SA failure during IKE_AUTH, as RFC 7296 Section 2.21.2
permits. Today Ze answers such a failure with an error notification and then deletes the
IKE SA. The goal has three parts. A responder keeps the authenticated IKE SA. The owner
loop runs that SA with no Child SA. A later CREATE_CHILD_SA exchange attaches one.

### This is a capability, and not a compliance defect

**Ze is conformant today. This spec is not a record of an outstanding violation.**

The clause is a MAY for the responder. RFC 7296 Section 2.21.2 says a responder MAY
include the authentication payloads beside the error notification. It never requires it.
The one obligation in the sentence binds the INITIATOR, and Ze already meets it.

`RFC7296-2.21.2-2` is committed at `rfc/short/rfc7296.md`. It is gated in both
polarities by `TestErrInitiatorSurvivesPiggybackedErrorNotify`
(`internal/component/ike/engine/rfc7296_notify_error_test.go`), recorded at
`ai/RFC-REQUIREMENTS.md` with its positive tag at
`internal/component/ike/engine/rfc7296_notify_error_test.go` and its negative tag at
`:406`. Both are unit tier, so `./le verify current mode full` runs them on every push.

The value of this spec is operational, and not a compliance gain. A peer whose Child SA
fails keeps its IKE SA. It retries the Child SA with one CREATE_CHILD_SA exchange, and
it does not repeat the full handshake.

### Provenance (do not delete)

Thomas ruled on 2026-08-01 that this work lands in a follow-up spec, and not in
the rfcgate-1b RFC 7296 pilot spec, which is near closure. The pilot made Ze SEND
the error notification where it once sent silence. That half is complete and proven.

The pilot left a comment at the exact refusal site that names this follow-up
(`internal/component/ike/engine/responder.go`). Read it before you start.

### The obligation, quoted verbatim

`rfc/full/rfc7296.txt:3298-3315`, with the page break at `:3303-3310`. The block below
is quoted text, and not a code sample. `ai/rules/writing.md` keeps
quoted external text verbatim, because a changed quotation is false evidence.

```
If authentication has succeeded in the IKE_AUTH exchange, the IKE SA
is established; however, establishing the Child SA or requesting
configuration information may still fail.  This failure does not
automatically cause the IKE SA to be deleted.  Specifically, a
responder may include all the payloads associated with authentication
(IDr, CERT, and AUTH) while sending error notifications for the
piggybacked exchanges (FAILED_CP_REQUIRED, NO_PROPOSAL_CHOSEN, and so
on), and the initiator MUST NOT fail the authentication because of
this.  The initiator MAY, of course, for reasons of policy later
delete such an IKE SA.
```

The `RFC7296-2.21.2-2` row carries the second half of this text. The first half, from
"If authentication has succeeded" to "cause the IKE SA to be deleted", carries no RFC
2119 keyword and is therefore not a checklist row.

## Required Reading

### Architecture Docs
- [ ] `internal/component/ike/engine/notify_error.go` - the error-notification sender the rfcgate-1b pilot added
  → Decision: the pilot sends the error notification and then sets `StateDead`. This
    spec changes only the second half.
  → Constraint: the pilot spec is closed, so its text is in git history only. Read the
    sender in the code rather than looking for the spec.
- [ ] `ai/rules/rfc-compliance.md` - what a MAY costs
  → Constraint: making Ze more conformant needs no permission. Choosing anything
    narrower than full compliance needs Thomas.
- [ ] `ai/rules/evidence.md` - the new state is a resource an attacker reaches
  → Constraint: a guard that neither denies nor speaks does not exist. The Child SA
    count is a guard here.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps
  → Constraint: prove the test FAILS when the behavior under test is reverted. A test
    that asserts an SA stays alive passes when the SA was never at risk.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - IKEv2. Row `RFC7296-2.21.2-2` sits at `:564`.
  → Constraint: the responder clause is a MAY (`rfc/full/rfc7296.txt:3301-3313`).
  → Constraint: the initiator MUST NOT fail authentication over the piggybacked error
    (`rfc/full/rfc7296.txt:3311-3314`).
  → Constraint: the initiator MAY later delete such an IKE SA for reasons of policy
    (`rfc/full/rfc7296.txt:3314-3315`).
  → Constraint: UNSUPPORTED_CRITICAL_PAYLOAD, INVALID_SYNTAX and AUTHENTICATION_FAILED
    are the only notifications that delete the IKE SA without a Delete payload
    (`rfc/full/rfc7296.txt:3317-3321`). NO_PROPOSAL_CHOSEN is not one of them.

**Key insights:** (minimal context to resume after compaction)
- The initiator half is done and proven. The responder half does not exist.
- Three structural gaps block the responder half, and they are listed in Current
  Behavior. Two of them are outside the IKE_AUTH code.
- An IKE SA with no Child SA carries no traffic. It is a state that only a later
  CREATE_CHILD_SA exchange makes useful.

## Current Behavior (MANDATORY)

**Source files read:** (verified in the working tree on 2026-08-01)
- [ ] `internal/component/ike/engine/responder.go` -
  `(*PeerSession).handleAuthRequest` calls `buildAuthResponse`. On an error it calls
  `respondAuthError` and sets `sa.State = StateDead`. The comment at `:566-569` names
  this spec's work.
- [ ] `internal/component/ike/engine/responder.go` - `respondAuthError` sends one
  SK-encrypted error notification. It does not cache the response, because the comment
  at `:865-866` states the IKE SA is about to die.
- [ ] `internal/component/ike/engine/notify_error.go` - `notifyForRefusal` maps the
  refusal to INVALID_SYNTAX, TS_UNACCEPTABLE or NO_PROPOSAL_CHOSEN.
- [ ] `internal/component/ike/engine/responder.go` -
  `(*PeerSession).finishResponderEstablish` takes a `*ChildSA` and stores it. It holds
  the only non-test write of `sa.State = StateEstablished` on the responder side, at
  `:847`. A search for `= StateEstablished` outside test files returns this line and
  `internal/component/ike/engine/fsm.go`, which is the initiator path.
- [ ] `internal/component/ike/engine/established.go` - `(*PeerSession).runEstablished`
  is the owner loop. Its responder branch at `:56-64` reads the Child SA and returns
  `errInvalidMessage` when it is nil, with the log line "responder established without a
  child SA".
- [ ] `internal/component/ike/engine/established.go` - the initiator branch of the
  same function calls `createFirstChildSA` with no test of what the response accepted.
- [ ] `internal/component/ike/engine/child.go` - `createFirstChildSA`. At `:194-202`
  it generates a random outbound SPI when `sa.ChildOutboundSPI` is zero.
- [ ] `internal/component/ike/engine/fsm.go` - `handleAuthResponse` sets
  `sa.ChildOutboundSPI` only from an SA payload in the response. A response that carries
  an error notification in place of SAr2 leaves the field at zero.
- [ ] `internal/component/ike/engine/fsm.go` - `handleAuthResponse`. It aborts on
  `NotifyAuthenticationFailed` at `:678-682`. It aborts on an unrecognized ERROR
  notification through `failIfUnrecognizedErrorNotify` at `:654`. Every recognized
  notification reaches the collecting walk and changes no state.
- [ ] `internal/component/ike/engine/fsm.go` - the accepted-offer check runs
  under `if childOffer != nil`. A response with no SA payload skips it.
- [ ] `internal/component/ike/wire/payload_notify.go` - `NotifyTypeRecognized` reads
  `notifyTypeNames`. That table holds NO_PROPOSAL_CHOSEN at `:83`, FAILED_CP_REQUIRED at
  `:89` and TS_UNACCEPTABLE at `:90`. All three are therefore recognized. None of them
  reaches the abort at `fsm.go`.
- [ ] `internal/component/ike/engine/inbound.go` -
  `handleCreateChildSAOwned` refuses a CREATE_CHILD_SA that is neither a Child rekey nor
  an IKE rekey. It answers NO_PROPOSAL_CHOSEN. The comment states that Ze does not create
  a new Child SA.
- [ ] `internal/component/ike/engine/inbound.go` - the same function. Its Child rekey
  responder path at `:346-390` reads the existing Child SA at `:347` and replaces it.
- [ ] `internal/component/ike/engine/fsm.go` - `(*PeerSession).runResponder` polls
  the SA and adopts it into `runEstablished` when it reaches `StateEstablished`. It
  removes the SA and resets on `StateDead` at `:266-270`.

**The three structural gaps.** Each one blocks the capability on its own:

| # | Gap | Producer |
|---|-----|----------|
| G-1 | No responder establish path with no Child SA | `finishResponderEstablish` (`internal/component/ike/engine/responder.go`) |
| G-2 | The owner loop refuses a Child-SA-free SA | `runEstablished` (`internal/component/ike/engine/established.go`) |
| G-3 | No exchange attaches a Child SA later | `handleCreateChildSAOwned` (`internal/component/ike/engine/inbound.go`) |

**The initiator defect this work must also settle.** The initiator survives the
piggybacked error notification, which RFC 7296 requires and
`TestErrInitiatorSurvivesPiggybackedErrorNotify` proves. It then reaches
`runEstablished`, which calls `createFirstChildSA` at `established.go` with no test of
whether the responder accepted a Child SA.

`sa.ChildOutboundSPI` is zero in that case (`fsm.go`). `createFirstChildSA`
therefore invents an outbound SPI (`child.go`). The result is a Child SA whose
outbound half the peer never allocated, and the traffic on that tunnel is lost.
**This is a static reading of the producers, and not a measurement.** Reproduce it before
the design phase closes.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every `test/ipsec/*.ci` stays green. The suite is listed in `internal/le/functional/suites.go`.
- Every scenario under `test/interop-ipsec/scenarios/` stays green.
- `RFC7296-2.21.2-2` keeps both polarities. `ai/rules/rfc-compliance.md` makes proof
  monotonic, so the initiator test must never lose a tag.
- AUTHENTICATION_FAILED, INVALID_SYNTAX and UNSUPPORTED_CRITICAL_PAYLOAD stay fatal
  (`rfc/full/rfc7296.txt:3317-3321`).
- A failed AUTH verification still deletes the IKE SA
  (`internal/component/ike/engine/responder.go`).

**Behavior to change:** (only what the user asked for)
- A Child SA refusal during IKE_AUTH keeps the authenticated IKE SA alive.
- The owner loop runs an SA that carries no Child SA.
- A CREATE_CHILD_SA request attaches the first Child SA to such an SA.
- The initiator installs no Child SA when the response accepted none.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire: an IKE_AUTH request whose AUTH verifies, and whose piggybacked Child SA request
  Ze refuses.
- Wire: a later CREATE_CHILD_SA request that carries no REKEY_SA notification and no KE
  payload.

### Transformation Path
1. AUTH verification. `verifyRemoteAuth` passes
   (`internal/component/ike/engine/responder.go`).
2. Child SA construction. `buildAuthResponse`
   (`internal/component/ike/engine/responder.go`) returns an error.
3. Classification. `notifyForRefusal` (`internal/component/ike/engine/notify_error.go`)
   picks the notification type.
4. Response. The response chain carries IDr, CERT, AUTH and the error notification,
   instead of the single notification `respondAuthError` sends today.
5. Establishment. A responder establish path adopts the SA with no Child SA (G-1).
6. Owner loop. `runEstablished` accepts the SA and runs the liveness and lifetime timers
   without a Child SA (G-2).
7. Attachment. A later CREATE_CHILD_SA request builds and installs the first Child SA
   (G-3).
8. Teardown. Local policy deletes the SA, which RFC 7296 permits
   (`rfc/full/rfc7296.txt:3314-3315`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire codec ↔ IKE engine | The IKE_AUTH response chain gains payloads beside the notification | No |
| Responder handler ↔ owner loop | A `StateEstablished` SA with a nil Child SA | No |
| IKE engine ↔ dataplane | No install runs until a Child SA is attached | No |
| IKE engine ↔ event bus | `SAUp` fires with no matching `ChildSAEvent` | No |
| IKE engine ↔ `show vpn ipsec sa` | An established SA with no Child SA row | No |

### Integration Points
- `finishResponderEstablish` (`internal/component/ike/engine/responder.go`) already
  separates the parallel and the direct case. The Child-SA-free case is a third case, and
  not a new function.
- `respondChildRekey` (`internal/component/ike/engine/rekey.go`) builds a replacement
  Child SA from a request. The attachment path in G-3 needs the same construction with no
  old Child SA.
- `createFirstChildSA` (`internal/component/ike/engine/child.go`) is the single
  builder of a first Child SA. Reuse it rather than writing a second.

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
| A-1 | `finishResponderEstablish` is the only responder path to `StateEstablished` | A search for `= StateEstablished` outside test files returns `internal/component/ike/engine/responder.go` and `fsm.go` only, 2026-08-01 | A second path establishes an SA this design never reaches | Rerun the search at the start of the design phase | unvalidated |
| A-2 | The owner loop needs no Child SA for its liveness and lifetime timers | `runEstablished` (`internal/component/ike/engine/established.go`), read for the nil branch only | The timers read Child SA fields and the loop needs a wider change | Read every field access in the loop body | unvalidated |
| A-3 | The initiator installs a Child SA with an invented outbound SPI when the response accepted none | `established.go`, `fsm.go` and `child.go`, read 2026-08-01 | The initiator defect does not exist and phase 4 is unnecessary | Drive a response with a piggybacked notification and read the installed SPI | unvalidated |
| A-4 | strongSwan keeps its IKE SA when its Child SA request is refused | Not yet read | The interop scenario cannot prove the capability against a peer | Read the strongSwan source for its IKE_AUTH error path | unvalidated |
| A-5 | strongSwan retries the Child SA with a CREATE_CHILD_SA exchange after such a refusal | Not yet read | G-3 has no interop counterpart and only a Ze-to-Ze test proves it | Read the strongSwan retry policy and build the scenario | unvalidated |
| A-6 | `test/interop-ipsec/` stays outside the automated tiers | `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md` | An interop tag becomes legal evidence | Rerun `./le rfc check` at landing time | unvalidated |
| A-7 | No committed RFC row needs to change | `RFC7296-2.21.2-2` is bound in both polarities at `ai/RFC-REQUIREMENTS.md` | The spec touches a gated row and the proof ratchet applies | Rerun `./le rfc check` after the change | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An unauthenticated peer accumulates Child-SA-free IKE SAs. Each one costs keys, a session and a table entry, and none of them carries traffic. This is the primary security risk of the whole spec | The SA table grows against a peer that never completes a Child SA | Only an AUTHENTICATED peer reaches the new state, because the change sits after `verifyRemoteAuth` (`internal/component/ike/engine/responder.go`). Add a bounded lifetime for a Child-SA-free SA, and delete it when the timer expires. `ai/rules/evidence.md` governs the bound |
| R-2 | The bounded lifetime deletes an SA whose peer was about to attach a Child SA | A peer that retries slowly never establishes a tunnel | Reset the timer on every CREATE_CHILD_SA attempt, and choose the bound from the peer retry interval, and not from a round number |
| R-3 | The new state reaches the owner loop before the loop tolerates it, so `runEstablished` returns `errInvalidMessage` and the SA dies anyway. The capability then looks present and does nothing | A functional test shows the SA established and then gone within one poll interval | Land G-2 before G-1. The phase order in Implementation Steps encodes this |
| R-4 | The test asserts the SA survives, and passes because nothing ever put it at risk. `ai/rules/interop-and-goal-validation.md` names this trap | None. The test is green | Mutation-verify each test. Restore the `StateDead` write and confirm the test reddens |
| R-5 | The initiator fix in phase 4 removes the Child SA install and breaks every existing tunnel, because the guard reads the wrong field | The whole `test/ipsec` suite reddens | Guard on the accepted offer that `handleAuthResponse` collected, and not on `ChildOutboundSPI`, which a legitimate exchange can also reach by another path. Confirm A-3 first |
| R-6 | The responder answers with IDr, CERT, AUTH and the notification, and a peer rejects the wider chain. RFC 7296 permits the wider chain, and a peer can still be strict | A strongSwan scenario stops establishing | Prove the chain against strongSwan before the phase closes. Keep the narrow chain reachable by configuration until the proof lands |
| R-7 | `respondAuthError` does not cache its response (`internal/component/ike/engine/responder.go`), because the SA was about to die. The SA now survives, so a retransmitted IKE_AUTH request finds no cached response | A peer that retransmits IKE_AUTH gets silence, and its own timer deletes the SA | Cache the response on the surviving path. This is a direct consequence of the state change, and it is easy to miss |
| R-8 | G-3 opens a new Child SA creation path, and that path skips a check the IKE_AUTH path performs. Traffic selectors and policy are the likely omission | A CREATE_CHILD_SA attaches a Child SA wider than the configured selectors | Route the attachment through the same narrowing the IKE_AUTH path uses (`internal/component/ike/engine/ts_narrow.go`) |
| R-9 | Engine line numbers move. Other agents edit `internal/component/ike/engine/` concurrently | A citation names a line holding different code | Every citation in this spec names its function. Relocate by function name before you quote a line |
| R-10 | A reader treats this spec as an RFC compliance fix and reopens `RFC7296-2.21.2-2` | A review or an audit reports the row as unproven | The Task section states the row is proven. Repeat that statement in the learned summary |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An IKE SA survives when Ze must delete it, or an authenticated peer accumulates SAs that carry no traffic. A wrong initiator guard removes the Child SA install from every tunnel, which is total data-path loss |
| How is it reverted? | A single commit revert, while the behavior stays off by default. Once a peer depends on the surviving SA, a revert deletes that SA on the next Child SA refusal |
| Who else touches this path? | the rfcgate-1b RFC 7296 pilot spec (the error notification sender, near closure), `plan/spec-ipsec-remote-access.md` (the same IKE_AUTH response chain and the CP payload), `plan/spec-ipsec-ipcomp.md` (the same Child SA negotiation), `spec-fixit-vpp-ipsec-inoperable` (the dataplane the attachment path installs into) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An IKE_AUTH request whose AUTH verifies and whose Child SA request Ze refuses | → | The refusal branch of `handleAuthRequest` (`internal/component/ike/engine/responder.go`) | `TestPiggybackResponderKeepsIKESAAlive` |
| A responder SA with no Child SA reaches the owner loop | → | `runEstablished` (`internal/component/ike/engine/established.go`) | `TestOwnerLoopRunsWithoutChildSA` |
| A CREATE_CHILD_SA request arrives on a Child-SA-free SA | → | `handleCreateChildSAOwned` (`internal/component/ike/engine/inbound.go`) | `TestCreateChildSAAttachesFirstChild` |
| An IKE_AUTH response carries an error notification in place of SAr2 | → | The initiator branch of `runEstablished` (`internal/component/ike/engine/established.go`) | `TestInitiatorInstallsNoChildSAWhenNoneAccepted` |
| An operator reads a Child-SA-free SA | → | `show vpn ipsec sa` (`internal/component/ike/cmd/show_ipsec.go`) | `test/ipsec/ipsec-show-sa-no-child.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer authenticates, and its piggybacked Child SA request is refused | The IKE SA reaches the established state, and the responder does not delete it |
| AC-2 | The same exchange | The IKE_AUTH response carries IDr, AUTH and the error notification in one chain |
| AC-3 | The same exchange, with a certificate configured | The response also carries the CERT payload |
| AC-4 | The same exchange | No Child SA is installed in the dataplane, and no route is announced |
| AC-5 | A Child-SA-free SA is established | The owner loop runs it, and the liveness exchange completes |
| AC-6 | The peer retransmits the IKE_AUTH request | The responder replies with the cached response, and the SA stays established |
| AC-7 | The peer sends a CREATE_CHILD_SA request with no REKEY_SA notification | Ze builds the first Child SA, installs it, and answers with SAr2 and TSr |
| AC-8 | The attached Child SA carries traffic | The traffic selectors match the configured selectors, and not the peer request |
| AC-9 | A Child-SA-free SA reaches its bounded lifetime with no attachment | Ze deletes the IKE SA and sends a Delete payload |
| AC-10 | The peer attempts to attach a Child SA before the bound expires | The timer resets, and the SA survives |
| AC-11 | An unauthenticated peer fails AUTH verification | The IKE SA is deleted, exactly as it is today |
| AC-12 | A response carries AUTHENTICATION_FAILED | The initiator deletes the SA, exactly as it is today |
| AC-13 | An IKE_AUTH response carries an error notification in place of SAr2 | The initiator installs no Child SA, and it invents no outbound SPI |
| AC-14 | The same response | The initiator keeps the IKE SA established, which `RFC7296-2.21.2-2` requires |
| AC-15 | An operator runs `show vpn ipsec sa` against a Child-SA-free SA | The output names the SA as established and reports no Child SA |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Connects a peer whose Child SA policy disagrees with Ze | wire → `handleAuthRequest` → refusal → established SA | `test/ipsec/ipsec-auth-piggyback-survives.ci` |
| 2 | Corrects the peer policy and retries the Child SA | wire → `handleCreateChildSAOwned` → `createFirstChildSA` → dataplane | `test/ipsec/ipsec-auth-piggyback-attach.ci` |
| 3 | Reads the state between the two steps | engine state → `show vpn ipsec sa` | `test/ipsec/ipsec-show-sa-no-child.ci` |
| 4 | Leaves the peer unattended past the bound | timer → Delete payload → SA removed | `test/ipsec/ipsec-auth-piggyback-expiry.ci` |
| 5 | Runs Ze as initiator against a strict peer that refuses the Child SA | wire → `handleAuthResponse` → established SA with no Child SA | `test/interop-ipsec/scenarios/auth-piggyback` |  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `skeleton` and the work is not implemented) -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPiggybackResponderKeepsIKESAAlive` | `internal/component/ike/engine/rfc7296_piggyback_test.go` | AC-1 | |
| `TestPiggybackResponseCarriesAuthPayloads` | `internal/component/ike/engine/rfc7296_piggyback_test.go` | AC-2 and AC-3 | |
| `TestPiggybackInstallsNoChildSA` | `internal/component/ike/engine/rfc7296_piggyback_test.go` | AC-4 | |
| `TestOwnerLoopRunsWithoutChildSA` | `internal/component/ike/engine/established_test.go` | AC-5 | |
| `TestPiggybackResponseIsCached` | `internal/component/ike/engine/rfc7296_piggyback_test.go` | AC-6 | |
| `TestCreateChildSAAttachesFirstChild` | `internal/component/ike/engine/rfc7296_piggyback_test.go` | AC-7 | |
| `TestAttachedChildSAUsesConfiguredSelectors` | `internal/component/ike/engine/rfc7296_piggyback_test.go` | AC-8 | |
| `TestChildFreeSAExpires` | `internal/component/ike/engine/rfc7296_piggyback_test.go` | AC-9 and AC-10 | |
| `TestAuthFailureStillDeletesSA` | `internal/component/ike/engine/rfc7296_piggyback_test.go` | AC-11 | |
| `TestInitiatorInstallsNoChildSAWhenNoneAccepted` | `internal/component/ike/engine/rfc7296_piggyback_test.go` | AC-13 and AC-14 | |

`TestErrInitiatorSurvivesPiggybackedErrorNotify`
(`internal/component/ike/engine/rfc7296_notify_error_test.go`) already proves AC-12
and the surviving half of AC-14. Extend it rather than writing a second test, and keep
both `RFC7296-2.21.2-2` tags in place. A behavior change to that function needs the
owner's approval, because the RFC-tagged-test hook blocks it.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Child-SA-free lifetime | design decision, in seconds | the configured bound | 0 disables the bound and needs a decision | a value above the IKE SA lifetime, which the shorter timer must win |
| Child SA count on an established SA | 0-1 | 1 | N/A | 2, which the attachment path must refuse |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-auth-piggyback-survives` | `test/ipsec/ipsec-auth-piggyback-survives.ci` | The tunnel fails, and the IKE SA stays up | |
| `ipsec-auth-piggyback-attach` | `test/ipsec/ipsec-auth-piggyback-attach.ci` | The peer retries and the tunnel comes up | |
| `ipsec-auth-piggyback-expiry` | `test/ipsec/ipsec-auth-piggyback-expiry.ci` | The unattended SA is deleted | |
| `ipsec-show-sa-no-child` | `test/ipsec/ipsec-show-sa-no-child.ci` | The operator reads the intermediate state | |

The `ipsec` suite runs inside `./le verify current mode full`, so a `.ci` there earns a verify tier. A `.ci`
that makes IKE_AUTH refuse the Child SA while AUTH still verifies needs a configuration
that disagrees on ESP only. `plan/deferrals/rfcgate-1b-rfc7296-pilot.md` records that a
disjoint `esp-group` fails `selectResponderESP` at IKE_AUTH, so the SA never establishes.
**Read that row before you design the fixture.** It blocked a sibling test, and it blocks
this one the same way.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `auth-piggyback` | `test/interop-ipsec/scenarios/` | strongSwan | A strongSwan peer whose Child SA Ze refuses keeps its IKE SA | |
| `auth-piggyback-attach` | `test/interop-ipsec/scenarios/` | strongSwan | The same peer attaches a Child SA with a later CREATE_CHILD_SA exchange | |

**These scenarios cannot carry an RFC tag.** `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md`
owns that constraint, because no automated caller runs the tree. Write the reason into
each scenario header, so a later reader does not add a tag. Scenario directories are
named, never numbered, so no number has to be reserved.

## Files to Modify
- `internal/component/ike/engine/responder.go` - the refusal branch of
  `handleAuthRequest`, `finishResponderEstablish`, and
  `respondAuthError` for the cached response
- `internal/component/ike/engine/established.go` - `runEstablished`, the nil Child
  SA branch, and the initiator branch
- `internal/component/ike/engine/inbound.go` - `handleCreateChildSAOwned`
- `internal/component/ike/engine/fsm.go` - the initiator record of the accepted offer
  (`:707-713`)
- `internal/component/ike/engine/child.go` - the attachment entry to `createFirstChildSA`
  (`:154`)
- `internal/component/ike/cmd/show_ipsec.go` - the Child-SA-free display
- `internal/component/ike/engine/rfc7296_notify_error_test.go` - the extended initiator
  test, with both `RFC7296-2.21.2-2` tags kept
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the bounded lifetime leaf, if
  the design phase makes it configurable

## Files to Create
- `internal/component/ike/engine/rfc7296_piggyback_test.go` - the unit tests
- `test/ipsec/ipsec-auth-piggyback-survives.ci`
- `test/ipsec/ipsec-auth-piggyback-attach.ci`
- `test/ipsec/ipsec-auth-piggyback-expiry.ci`
- `test/ipsec/ipsec-show-sa-no-child.ci`
- `test/interop-ipsec/scenarios/auth-piggyback/`  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `skeleton` and the work is not implemented) -->
- `test/interop-ipsec/scenarios/auth-piggyback-attach/`  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `skeleton` and the work is not implemented) -->

### Proposed Config Surface

Expressed as a table, because `ai/rules/spec-no-code.md` forbids code in a spec. The
final shape is a design-phase decision.

| Path | Type | Default | Units | Purpose |
|------|------|---------|-------|---------|
| `vpn ipsec ike-group <name> child-sa-grace` | uint32 | design decision | seconds | How long an authenticated IKE SA lives with no Child SA |

The name follows `ai/rules/config.md`: kebab-case, no abbreviations, and the unit
in a YANG `units` statement rather than in the name. The design phase must answer whether
the bound belongs in the config tree at all, and whether a value of 0 means "delete at
once" or "no bound". `ai/rules/config.md` decides that question, and the default
answer it gives is a YANG leaf.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, the bounded lifetime leaf |
| YANG validation constraints | Yes | The leaf takes a `range` and a `units` statement |
| YANG custom validators | No | The leaf takes native YANG constraints, and no runtime set is involved |
| CLI commands/flags | No | The feature is config-driven. `show vpn ipsec sa` gains a state, and that command exists |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | Yes | Automatic for a typed leaf. Confirm during the design phase |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-show-sa-no-child.ci` |
| Pipe completeness | Yes | `show vpn ipsec sa` already routes through the pipe framework. The new state must survive `\| json` |
| Env var registration | No | No leaf under `environment/` is added |
| Doctor check for runtime dependencies | No | The change adds no file, socket, port, module or binary |
| Prometheus counters/metrics | Yes | A counter of Child-SA-free established SAs. It is the observable signal for R-1. Name it during the design phase |
| BGP family surface (new SAFI / capability / attribute) | N-A | This is IKEv2 and IPsec, and not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, if the bound becomes a leaf |
| 3 | CLI command added/changed? | No | `show vpn ipsec sa` gains a state, and row 6 covers its page |
| 4 | API/RPC added/changed? | No | No new RPC |
| 5 | Plugin added/changed? | No | The IKE plugin registration shape is unchanged |
| 6 | Has a user guide page? | Yes | The IPsec operator guide. Confirm the path during the design phase |
| 7 | Wire format changed? | Yes | The IKE_AUTH response chain carries more payloads. Confirm the target page |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md`. Its row for RFC 7296 gains coverage, and `RFC7296-2.21.2-2` keeps its existing proof |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, for any new IKEv2 fixture |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`. strongSwan keeps the IKE SA in this case |
| 12 | Internal architecture changed? | Yes | The IPsec subsystem architecture page, for the new SA state |
| 13 | Route metadata keys added/changed? | No | No route metadata is involved |
| 14 | Prometheus counters added/changed? | Yes | The counter named in the Integration Checklist |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing new registers |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for anchors naming the files in Files to Modify |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify every IPsec state example against the changed engine |

## Implementation Steps

Phase 2 lands before phase 3 on purpose. R-3 records the reason: an SA that reaches an
intolerant owner loop dies anyway, and the capability then looks present and does nothing.

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: every row of the Wiring Test table
   - Files: `internal/component/ike/engine/rfc7296_piggyback_test.go`,
     `internal/component/ike/engine/established_test.go`
   - Verify: each test fails, and each failure names the gap it covers
2. **Phase: Owner loop tolerance (G-2)**
   - Tests: `TestOwnerLoopRunsWithoutChildSA`, AC-5
   - Files: `internal/component/ike/engine/established.go`
   - Verify: the loop runs a Child-SA-free SA, and the liveness exchange completes. A-2
     is confirmed or broken by this phase
3. **Phase: Responder establish path (G-1)**
   - Tests: AC-1 through AC-4, AC-6, AC-11
   - Files: `internal/component/ike/engine/responder.go`
   - Verify: the SA survives the refusal, the response carries the authentication
     payloads, and the response is cached. R-7 is settled here
4. **Phase: Initiator correctness**
   - Tests: AC-13, AC-14, and the extended
     `TestErrInitiatorSurvivesPiggybackedErrorNotify`
   - Files: `internal/component/ike/engine/established.go`,
     `internal/component/ike/engine/fsm.go`
   - Verify: A-3 is reproduced first. The initiator installs no Child SA when none was
     accepted, and it keeps the IKE SA. Both `RFC7296-2.21.2-2` tags stay
5. **Phase: Attachment (G-3)**
   - Tests: AC-7, AC-8
   - Files: `internal/component/ike/engine/inbound.go`,
     `internal/component/ike/engine/child.go`
   - Verify: a CREATE_CHILD_SA request attaches the first Child SA, and the selectors
     come from the configuration
6. **Phase: The bound and its observability**
   - Tests: AC-9, AC-10, AC-15
   - Files: `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`,
     `internal/component/ike/engine/established.go`,
     `internal/component/ike/cmd/show_ipsec.go`
   - Verify: an unattended SA is deleted, an attempt resets the timer, and the operator
     reads the state. R-1 is closed here
7. **Phase: Interoperability and documentation**
   - Tests: scenarios `auth-piggyback` and `auth-piggyback-attach`,
     `./le doc-check verify`, `./le rfc check`
   - Files: the two scenario directories, the documentation checklist rows
   - Verify: both strongSwan scenarios pass, and no RFC row lost a polarity

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at `file:line` |
| Feature completeness | Every user story has a working path, and no broken links |
| Correctness: the fatal set is unchanged | AUTHENTICATION_FAILED, INVALID_SYNTAX and UNSUPPORTED_CRITICAL_PAYLOAD still delete the IKE SA (`rfc/full/rfc7296.txt:3317-3321`) |
| Correctness: only an authenticated peer reaches the new state | The change sits after `verifyRemoteAuth` (`internal/component/ike/engine/responder.go`), and no earlier branch reaches it |
| Correctness: the bound is enforced | A Child-SA-free SA cannot live past the bound by any path, including a peer that keeps the session busy |
| Correctness: one Child SA | The attachment path refuses a second Child SA on an SA that already carries one |
| Mutation: restore the `StateDead` write in the refusal branch | AC-1 reddens. If it stays green the test never put the SA at risk (R-4) |
| Mutation: restore the `errInvalidMessage` return in the owner loop | AC-5 reddens |
| Mutation: remove the response cache on the surviving path | AC-6 reddens |
| Mutation: let the attachment path read the peer selectors | AC-8 reddens |
| Mutation: remove the initiator guard | AC-13 reddens, and the invented SPI returns |
| Mutation: disable the bound | AC-9 reddens |
| Naming | The YANG leaf is kebab-case with no abbreviations, and its unit is a `units` statement |
| Data flow | No dataplane call runs for a Child-SA-free SA, on any path |
| Rule: `ai/rules/evidence.md` | The bound denies by default. An unset or unreadable bound never means "no limit" |
| Rule: `ai/rules/rfc-compliance.md` | No answer narrower than full implementation with full proof was chosen. Thomas answered every such question |
| Rule: `ai/rules/evidence.md` | Every claim about the engine cites the producing function, and not its caller |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The IKE SA survives a Child SA refusal | `test/ipsec/ipsec-auth-piggyback-survives.ci` passes |
| A Child SA attaches later | `test/ipsec/ipsec-auth-piggyback-attach.ci` passes |
| The bound is enforced | `test/ipsec/ipsec-auth-piggyback-expiry.ci` passes |
| The initiator invents no SPI | `TestInitiatorInstallsNoChildSAWhenNoneAccepted` passes |
| `RFC7296-2.21.2-2` keeps both polarities | `./le rfc check` passes, and `ai/RFC-REQUIREMENTS.md` still shows both columns filled |
| The ledger is fresh | `./le rfc index-update` produces no diff |
| No interop tag was added | `grep -rn 'RFC requirement:' test/interop-ipsec/` returns nothing |
| Interoperability is proven | Scenarios `auth-piggyback` and `auth-piggyback-attach` pass |
| The operator can read the state | `test/ipsec/ipsec-show-sa-no-child.ci` passes |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | The primary risk. An authenticated peer that never attaches a Child SA must not accumulate IKE SAs. The bound in phase 6 is the control, and the counter is its signal |
| Authorization | Only a peer that passed `verifyRemoteAuth` reaches the new state. Confirm that no earlier branch, including the EAP path, reaches it |
| Fail closed | An unreadable or unset bound must deny rather than grant unlimited life |
| Input validation | The CREATE_CHILD_SA attachment path parses a peer request that no earlier exchange constrained. Validate the proposal, the selectors and the payload count |
| State confusion | An SA with no Child SA must never reach a dataplane call, a route announcement or a traffic counter |
| Error leakage | The wider IKE_AUTH response carries IDr and CERT to a peer whose Child SA failed. The peer is authenticated, so this is the payload set RFC 7296 names |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The capability is three changes in three files, and only one of them sits in the
  IKE_AUTH code. A reader who plans this work from the IKE_AUTH site alone misses G-2 and
  G-3, and delivers a state that dies on the next poll.
- An IKE SA with no Child SA carries no traffic, so G-3 is what makes the capability
  useful. Without an attachment path the operator gains a live SA and no tunnel.
- The initiator half is already conformant and proven, and it still holds a defect. The
  two facts are compatible: RFC 7296 requires the initiator not to fail authentication,
  and it says nothing about the Child SA the initiator then installs.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Land the capability in this spec, and not in the pilot | Implement it inside the rfcgate-1b pilot | Owner ruling, 2026-08-01. The pilot is near closure, and this work touches three files it does not own |
| Treat the work as a capability, and not as a compliance fix | Reopen `RFC7296-2.21.2-2` | The responder clause is a MAY (`rfc/full/rfc7296.txt:3301-3313`), and the row is proven in both polarities (`ai/RFC-REQUIREMENTS.md`) |
| Bound the life of a Child-SA-free SA | Let it live for the full IKE SA lifetime | R-1. The state is reachable by any authenticated peer, and it carries no traffic |

## Known Limitations

- The capability needs a peer that retries with CREATE_CHILD_SA. A-5 records that Ze has
  not yet confirmed strongSwan does this. If it does not, the surviving SA helps only a
  Ze peer and an operator who repairs the configuration.
- The design does not add a Ze-initiated attachment. Ze answers a CREATE_CHILD_SA
  request, and it does not send one for a first Child SA. Anything outstanding here needs
  a row in `plan/deferrals/ipsec-auth-piggyback.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

The refusal branch, the owner loop tolerance and the attachment path each need a comment
citing RFC 7296 Section 2.21.2. Each comment must state that the responder clause is a
MAY, so a later reader does not read the code as a mandatory behavior.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify current mode full` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/speclifecycle/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

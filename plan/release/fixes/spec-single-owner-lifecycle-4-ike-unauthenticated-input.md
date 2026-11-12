# Spec: single-owner-lifecycle-4-ike-unauthenticated-input

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | A shipped-daemon defect an operator meets in production: one datagram built from an observed IKE header ends an initiator's handshake, against a MUST NOT of an RFC Ze implements. |
| RFC gaps | RFC7296-2.4-3 |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |
| WIP cap | Claimed above the cap (17/17) on Thomas's order for this spec, 2026-11-11: "ignore the cap" |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 4 of `plan/release/fixes/spec-single-owner-lifecycle.md`, which holds the
decision record and the row routing. This child owns one journal row:
`plan/journal/unprotected-message-changes-sa-state.md`, 2026-09-29.

The problem. An IKE message that fails its integrity check carries no proof it
came from the peer: both SPIs and the Message ID travel in the clear, so anyone
who saw one datagram of the exchange can build it. The responder already drops
such a message during the handshake and keeps the SA (fixed 2026-11-09). The
initiator does not: `handleAuthResponse` and `handleEAPResponse`
(`internal/component/ike/engine/fsm.go`) call `decryptSKPayload` themselves and
set `StateDead` on any error it returns, so one forged IKE_AUTH response ends
the handshake, `runInitiator` returns `errSADead`, and the tunnel waits out a
reconnect backoff.

RFC 7296 Section 2.4: "Since IKE is designed to operate in spite of DoS attacks
from the network, an endpoint MUST NOT conclude that the other endpoint has
failed based on any routing information (e.g., ICMP messages) or IKE messages
that arrive without cryptographic protection (e.g., Notify messages complaining
about unknown SPIs)."

The symptom an operator sees: a tunnel that Ze initiates flaps or never comes
up while a third party, or a path that damages datagrams, feeds it IKE_AUTH
responses under the live SPIs. Nothing names the cause above debug level.

The goal. Every SK-protected inbound message, in every role and state, is
opened by one function. That function consumes the unauthenticated failure
itself (drops the datagram, counts it, logs at debug), so no caller holds an
error it could turn into a state change. A message whose integrity check
verified and whose content is malformed is the peer's own, and keeps the
outcome it has today. An initiator that received only forgeries keeps
retransmitting on its ordinary schedule, and the peer's genuine response still
establishes the same SA. `ze doctor` names a handshake that is receiving
messages which fail their integrity check.

The owner's decisions this rests on, in his words (2026-11-11, recorded in the
parent spec): "D1 - yes you are right" (no annotation gate; each fix closes its
bypass at a choke point) and "D3 - Ok add it to the post-release spec" (the
single transition function for `sa.State` leaves this fix).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: a protocol peer is an outside party, and the order of authority is the RFC first. A readiness or problem report is a doctor check registered by the package that owns the state.
- [ ] `ai/doors/access-security.md` - the IKE door
  → Constraint: readers of IKE state outside the engine use the two core leaves, never `ike/engine`. The doctor check this spec adds lives in `ike/engine` and is declared in `Registration.DoctorChecks` (`engine/register.go`), like its siblings.
  → Constraint: the door's trap paragraph "A datagram that fails to decrypt kills a live SA" says the initiator half is open. It becomes false in the phase that fixes the initiator and is edited in that phase.
  → Constraint: a plain `go test` of a package that links `internal/core/eap` does not compile. `ike/engine` links it, so its tests run through `./le`.
- [ ] `ai/doors/le-tooling.md` - the door for `test/` and `internal/le/interoplab`
  → Constraint: a `.ci` is drafted under `test/draft/ipsec/` and promoted; a compiled fixture reports a failed assertion by returning an error; an interop checker change whose scenario holds a recording owes a replay test.
- [ ] `ai/doors/core.md` - the door for `internal/core/diagnostic`
  → Constraint: core imports nothing upward. The new diagnostic code is one constant in `internal/core/diagnostic/codes.go`, and the check that raises it stays in `ike/engine`.
- [ ] `docs/architecture/ike/ipsec-14-responder.md`, "An IKE_AUTH that does not decrypt must not end the handshake" - the responder half as landed
  → Decision: the drop sends nothing, writes no SA field (state, Message ID window, `CreatedAt`), counts under `ze_ipsec_error_notify_suppressed_total{reason="handshake-unauthenticated"}` and logs at debug only, because a WARN line per forged datagram lets an off-path sender write the operator's log.
  → Constraint: `errInnerParse` is the one decrypt failure that is evidence about the peer. It is set by `decryptSKPayload` for a CBC failure after the checksum verified (Padding, Pad Length, block alignment) and by `decryptAndParse` for an inner chain that does not parse.
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, "An EAP message whose Length exceeds its octets" and "An unprotected IKE_AUTH response is dropped"
  → Constraint: `eapMessageDiscarded` is asked before any other verdict on a parse error. It matches through wrapping (`errors.Is` on `wire.ErrEAPLengthExceedsData`), so it keeps working on an error the entry marks `errInnerParse`. Such a message is dropped and the SA kept: RFC 3748 Section 4.
  → Constraint: an IKE_AUTH response with no SK payload is already dropped by both initiator handlers, with nothing counted.
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the initiator loop and the second IKE_AUTH exchange
  → Constraint: "A second-exchange message that fails to decrypt is dropped and the SA kept." This sentence covers every `decryptSKPayload` error today, the authenticated CBC padding failure included. This spec keeps that: it changes the reaction to unauthenticated input only.
- [ ] `ai/digests/ipsec-ike.md` - the current flow
  → Constraint: handshake responses are handled on the dispatch goroutine (`handleInbound`), which wakes `runInitiator` through `wakeHandshake`; established-SA messages are handled on the owner loop (`handleOwnedInbound`).
- [ ] `docs/contributing/rfc-conformance-gates.md` - tag and discrimination-record obligations
  → Constraint: a tagged unit this change adds owes a discrimination record in the same change (`./le rfc discriminate`, `./le rfc record`); a recorded proof is fingerprinted on its producer, so every record whose producer this change edits goes stale and owes a re-record.
- [ ] `docs/architecture/doctor-and-health-checks.md`, `ai/rules/user-facing-errors.md` - what a doctor check owes
  → Constraint: read both in full before the doctor phase. This spec's author did not; the check's message text is specified here by content only.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - row `RFC7296-2.4-3` and row `RFC7296-2.4-12`
  → Constraint: `RFC7296-2.4-3` is annotated `{partial}` with gap "an endpoint", naming `handleAuthResponse` and `handleEAPResponse`, and names `plan/release/fixes/spec-single-owner-lifecycle.md` as owner. The annotation is re-judged by an independent audit after the fix, never edited by the author of the fix.
  → Constraint: `RFC7296-2.4-12`, "Implementations MUST limit the rate at which they take actions based on unprotected messages", is met for these paths by the inbound token bucket in `dispatchInbound`. The drop path adds no action that bypasses it.
- [ ] `rfc/full/rfc7296.txt`, Section 2.4, Section 2.21.2, Section 2.21.4, Section 3.1
  → Constraint: Section 2.4 is the deciding sentence, quoted whole in Task. Section 2.21.1 is "Error Handling in IKE_SA_INIT" and is not cited for IKE_AUTH. The parent spec's quote that begins "since the first two messages of an SA setup" is a fragment about IKE_SA_INIT and is not reused.

**Key insights:**
- The defect is two call sites, not a missing mechanism: the responder's drop exists and the initiator bypasses it.
- An established SA already survives garbage at every site. Three of those sites count nothing.
- `runInitiator` needs no change: a handler that returns without writing state leaves the retransmit schedule as it was.
- strongSwan does the same thing in `parse_message`: an integrity failure is ignored and the SA kept.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/fsm.go` - `runInitiator`, `awaitHandshake`, `handleInbound`, `handleAuthResponse`, `handleEAPResponse`, `sendEAPResponsePacket`, the retransmit constants
- [ ] `internal/component/ike/engine/inbound.go` - `handleOwnedInbound`, `decryptAndParse`, `errInnerParse`
- [ ] `internal/component/ike/engine/auth.go` - `decryptSKPayload`
- [ ] `internal/component/ike/engine/responder_unauthenticated.go` - `discardUnauthenticatedHandshakeRequest`, `suppressedUnauthenticatedHandshake`
- [ ] `internal/component/ike/engine/responder.go` - `handleAuthRequest`
- [ ] `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP`
- [ ] `internal/component/ike/engine/multiple_auth.go` - `handleResponderAuthRound`
- [ ] `internal/component/ike/engine/held_successor.go` - `handleHeldSuccessorInbound`
- [ ] `internal/component/ike/engine/eap_auth.go` - `eapMessageDiscarded`
- [ ] `internal/component/ike/engine/metrics.go` - `countErrorNotifySuppressed`, `errorNotifySuppressedCount`, `publishErrorNotifyCounts`, `RegisterMetrics`
- [ ] `internal/component/ike/engine/doctor_virtual_ip.go`, `doctor_cookie.go`, `register.go` (`Registration.DoctorChecks`) - a check over the running engine, a check over the config tree, and how both are declared
- [ ] `internal/component/ike/engine/sa.go` - the `SA` struct head (`State`, the SPIs)
- [ ] `internal/component/ike/crypto/aead.go` - `OpenIKEAEAD`
- [ ] `internal/core/diagnostic/codes.go` - the `doctor-ipsec-*` codes
- [ ] `internal/le/interoplab/ipsec/unprotected_eap.go`, `checkers.go` - `checkResponderEAPSurvivesUnprotectedAuth` and the scenario table
- [ ] strongSwan 6.1.0 `src/libcharon/sa/ikev2/task_manager_v2.c` - `parse_message`, `process_message`; `src/libcharon/processing/jobs/process_message_job.c` - `execute`

What each caller does today. `decryptSKPayload` has three callers in non-test
engine files and `decryptAndParse` has eight. Every one is in this table.

| Caller (file, function, situation) | Calls | Unauthenticated failure today | Authenticated malformed today |
|------------------------------------|-------|-------------------------------|-------------------------------|
| `fsm.go` `handleAuthResponse`, first IKE_AUTH response, initiator | `decryptSKPayload` | WARN line, `StateDead`. A response with no SK payload is dropped at debug before the call, uncounted | CBC padding failure: `StateDead`. Chain parse failure: `StateDead`, except an EAP Length discard, which keeps the SA |
| `fsm.go` `handleAuthResponse`, RFC 4739 second exchange | `decryptSKPayload` | WARN line, SA kept, uncounted | CBC padding failure: WARN line, SA kept. Chain parse failure: `StateDead` |
| `fsm.go` `handleEAPResponse`, each EAP round, initiator | `decryptSKPayload` | WARN line, `StateDead`. No SK payload: dropped at debug, uncounted | `StateDead`, except an EAP Length discard |
| `inbound.go` `decryptAndParse` | `decryptSKPayload` | returns the error unmarked | returns it marked `errInnerParse` |
| `responder.go` `handleAuthRequest` | `decryptAndParse` | dropped, counted `handshake-unauthenticated`, debug | WARN, `StateDead`, nothing sent |
| `responder_eap.go` `handleResponderEAP` | `decryptAndParse` | dropped, counted `handshake-unauthenticated`, debug | EAP Length discard keeps the SA; otherwise WARN, `StateDead` |
| `multiple_auth.go` `handleResponderAuthRound` | `decryptAndParse` | dropped, counted `handshake-unauthenticated`, debug | WARN, `StateDead` |
| `inbound.go` `handleOwnedInbound`, retransmit arm on a MOBIKE SA | `decryptAndParse` | dropped, SA kept, uncounted, no log | dropped, SA kept |
| `inbound.go` `handleOwnedInbound`, out-of-window INFORMATIONAL response | `decryptAndParse` | falls to the out-of-window debug line, SA kept, uncounted | same |
| `inbound.go` `handleOwnedInbound`, out-of-window request | `decryptAndParse` | counted `invalid-msgid-unauthenticated`, SA kept | counted the same, SA kept, no INVALID_MESSAGE_ID |
| `inbound.go` `handleOwnedInbound`, in-window request or response | `decryptAndParse` | debug line, SA kept, uncounted, no liveness credit | a request draws INVALID_SYNTAX or UNSUPPORTED_CRITICAL_PAYLOAD through `respondInnerParseError`; a response is dropped |
| `held_successor.go` `handleHeldSuccessorInbound` | `decryptAndParse` | counted `held-successor-decrypt-failed`, SA kept | answered through `respondInnerParseError`, then `dropDeadHeldSuccessor` |

Rekey and MOBIKE handlers (`handleCreateChildSAOwned`, `handleInformationalOwned`,
`handleMobikeResponse`) decrypt nothing: `handleOwnedInbound` hands them the
chain it already opened.

What follows a dropped response on the initiator. `handleInbound` defers
`wakeHandshake`, so `runInitiator` runs one pass of its loop: the state is
neither `StateDead` nor `StateEstablished`, `RetransmitTime` is unchanged, and
it waits again. At each `RetransmitTime` it resends `LastSentMsg` (the
IKE_AUTH request, or the last EAP response, which `sendEAPResponsePacket`
stores) and returns `errTimeout` once `RetransmitCount` reaches
`maxRetransmissions`.

**Behavior to preserve:**
- The responder's handshake drop at its three sites, with its reason `handshake-unauthenticated`, its debug-only log and nothing written to the SA. The four tests of `rfc7296_unprotected_handshake_test.go` that carry or support `RFC7296-2.4-3` stay unedited and green.
- Authenticated malformed content keeps what that site does today, site by site: the "Authenticated malformed today" column of the table above is the preservation list. That includes the RFC 4739 second exchange keeping the SA on a CBC padding failure and ending it on a chain that does not parse, and an established SA answering a malformed request with INVALID_SYNTAX (which `respondError` follows with `StateDead`) or UNSUPPORTED_CRITICAL_PAYLOAD.
- `errInnerParse` and what it means; the EAP Length discard.
- The reasons `invalid-msgid-unauthenticated` and `held-successor-decrypt-failed`, with the counts they produce today.
- In `handleInbound`, `adoptAuthenticatedEndpoint` and `recordMobikeLocal` run only when `handleAuthResponse` left the SA in `StateEstablished`: "The state IS the authentication signal". The SA is established at one place, after `verifyRemoteAuth`.
- `runInitiator`'s loop, its retransmit schedule and `errTimeout`.
- No liveness credit for an unauthenticated message on an established SA.
- Every byte Ze emits.

**Behavior to change:**
- The initiator keeps its handshake when an IKE_AUTH response, in the first exchange, the second exchange or an EAP round, fails its integrity check or carries no SK payload. The drop is counted and logged at debug.
- Every caller opens an SK-protected message through one function, which replaces `decryptAndParse` and `discardUnauthenticatedHandshakeRequest`.
- The three established-SA sites that count nothing count their drop.
- A doctor check reports a peer whose handshake in progress has dropped messages that failed their integrity check.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| SK decryption primitive | `ike/engine/auth.go` `decryptSKPayload` | Verifies the AEAD tag or the CBC checksum, decrypts, strips CBC padding; marks its post-checksum failures `errInnerParse` | Does not change it. Its callers shrink to one, the entry |
| Decrypt and parse | `ike/engine/inbound.go` `decryptAndParse` | Finds the SK payload, calls the primitive, parses the inner chain, marks a parse failure `errInnerParse`, returns every failure as an error | Replaces it: the entry does these steps and consumes the unauthenticated failure instead of returning it |
| Handshake drop for a request | `ike/engine/responder_unauthenticated.go` `discardUnauthenticatedHandshakeRequest` | Classifies an error from `decryptAndParse`: counts and logs the unauthenticated ones, answers false for `errInnerParse` | Replaces it: absorbed into the entry. Its name and log text say "request", which is false for a response |
| The authenticated-malformed mark | `ike/engine/inbound.go` `errInnerParse` | Separates a failure after the integrity check from one before it | Extends its use: it is the only error the entry can return |
| EAP Length discard | `ike/engine/eap_auth.go` `eapMessageDiscarded` | Keeps the SA for an authenticated message whose EAP Length exceeds its octets | Does not change it. Callers keep asking it first, on the entry's error |
| Suppressed-notify counter | `ike/engine/metrics.go` `countErrorNotifySuppressed`, series `ze_ipsec_error_notify_suppressed_total{reason}` | Counts each emission a guard stopped, under the guard's name | Extends it: the entry requires a reason from its caller, and one reason is added for the established sites that count nothing |
| Initiator retransmit loop | `ike/engine/fsm.go` `runInitiator`, `awaitHandshake`, `retransmitBackoff` | Resends `LastSentMsg` with exponential backoff, gives up after `maxRetransmissions` | Does not change it. The drop relies on it, and a test pins that |
| Inbound rate limit | `ike/engine/register.go` `dispatchInbound` token bucket | Bounds how often any handler runs on unauthenticated input | Does not change it |
| Authentication signal | `ike/engine/fsm.go` `handleInbound`, the `StateAuthSent` arm | Adopts the peer endpoint only when the handler established the SA | Does not change it. A dropped message writes no state, so the signal stays exact |
| Doctor check over the running engine | `ike/engine/doctor_virtual_ip.go` `checkIPsecVirtualIPPool`, declared in `engine/register.go` `Registration.DoctorChecks` under the name `ipsec-virtual-ip-pool`, phase `rpc.DoctorPhasePostConfig` | Reads runtime state through `ActiveTable()` and reports a warning with a code from `internal/core/diagnostic/codes.go` | Extends the list with one check of the same shape |
| Interop checker for the responder half | `internal/le/interoplab/ipsec/unprotected_eap.go` `checkResponderEAPSurvivesUnprotectedAuth`, `suspicious.go` `injectOutOfSA` | Holds answers back with an iptables rule, injects forged datagrams under the live SPIs, asserts an exact counter rise and a completed exchange | Extends it with a sibling checker for the initiator half that reuses the injection route and the counter reader |
| One transition function for `sa.State` | none exists: `SA.State` is an exported field written at about a hundred sites | - | Does not apply. Owner decision D3 moved it to `plan/post-release/engineering/spec-validated-construction-and-state-types.md` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A UDP datagram on the IKE socket or the NAT-T socket (`transport.UDPTransport.Recv`), after the non-ESP marker is stripped.
- Format at entry: an RFC 7296 Section 3.1 header in the clear, then a payload chain whose Encrypted payload holds everything else.

### Transformation Path
1. `dispatchInbound` or `dispatchNATTInbound` (`engine/register.go`): token bucket, version and SPI checks, SA lookup by SPI pair.
2. Routing: an established SA with an owner loop goes to `PeerSession.inbound` and `handleOwnedInbound`; everything else goes to `handleInbound` on the dispatch goroutine, which routes a responder SA to `handleResponderInbound` (or `handleHeldSuccessorInbound`) and an initiator SA by `sa.State`.
3. The handler calls the one entry with the SA, the parsed message, the raw octets and the reason its drop is counted under.
4. The entry finds the SK payload, verifies and decrypts it through `decryptSKPayload`, and parses the inner chain. It answers one of three outcomes (table below).
5. The handler acts on an opened chain or on an authenticated-malformed error. On a dropped message it returns at once.

The entry's three outcomes:

| Outcome | When | What the entry has done | What the caller receives |
|---------|------|-------------------------|--------------------------|
| Opened | The integrity check verified and the chain parsed | Nothing else | The inner payload chain |
| Dropped | No SK payload, a message too short for a checksum, an associated-data span out of range, a checksum or AEAD tag that fails | Counted one under the caller's reason; one debug line naming the peer, the exchange type, request or response, the Message ID and the SA state; no field of the SA written except the drop count the doctor reads; nothing sent | The fact that the message was dropped, and no error value |
| Authenticated and malformed | The integrity check verified and the Padding, Pad Length, block alignment or inner chain does not parse | Nothing | An error that wraps `errInnerParse` and the cause, so `eapMessageDiscarded` and `wire.CriticalPayloadType` still read it. The caller can tell which step failed, the decrypt step (Padding, Pad Length, block alignment) or the chain parse, because the second exchange reacts differently to each today. Each caller keeps what that site does today |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer ↔ Ze | IKE datagrams, RFC 7296 | No |
| Dispatch goroutine ↔ `runInitiator` | `sa.State` and `wakeHandshake` | No |
| Engine ↔ metrics registry | `publishErrorNotifyCounts` copies the counters into the gauge | No |
| Plugin ↔ hub doctor | `rpc.DoctorCheckDiagnostic` from the declared check | No |

### Changed-Wire Census

No byte Ze emits changes. The source-supported reason: the drop path sends
nothing (`discardUnauthenticatedHandshakeRequest` and the four established
sites return without a send, and the initiator handlers return before any
send), and the retransmission `runInitiator` then sends is `LastSentMsg`
unchanged. What a peer observes is silence followed by the retransmission of
the request it already answered, where today it observes a new IKE_SA_INIT
after the reconnect backoff. Two machine-read surfaces do gain a value, and
they are the rows below.

**Search evidence:** scope: the whole repository except `vendor/`, `tmp/` and
`bin/`, all file types. Terms: `decryptSKPayload`, `decryptAndParse`,
`discardUnauthenticatedHandshakeRequest`, `errInnerParse` (callers);
`error_notify_suppressed`, `handshake-unauthenticated`,
`invalid-msgid-unauthenticated`, `held-successor-decrypt-failed` (the series
and its reasons). Positive control: the caller search returned the known
responder site `handleAuthRequest` (`responder.go`), and the series search
returned the known Python reader embedded in `unprotected_eap.go`. Findings:
callers are the twelve rows of Current Behavior; the series is written in
`metrics.go` and read in `unprotected_eap.go`, in two engine test files, and
named in `docs/guide/ipsec.md`, `docs/architecture/ike/ipsec-14-responder.md`,
`rfc/audit/rfc7296/RFC7296-2.4-3.json` and one website change fragment. No
shell or standalone Python file reads it.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| IKE datagram, IKE_AUTH request retransmission | writer | `ike/engine/fsm.go` `runInitiator` through `sendRaw` | resends `sa.LastSentMsg` at `RetransmitTime` | Preserved: no edit to the loop; pinned by `TestInitiatorKeepsRetransmittingAfterUnprotectedResponse` |
| IKE datagram, IKE_AUTH request retransmission | reader | strongSwan `task_manager_v2.c` `process_message` | a request at the previous Message ID is answered from `responding.packets` | Preserved: the peer sees a retransmission it already handles; proven by the interop scenario |
| `ze_ipsec_error_notify_suppressed_total{reason}` | writer | `ike/engine/metrics.go` `publishErrorNotifyCounts`, fed by `countErrorNotifySuppressed` | publishes one series per reason counted | Changes: one reason value added (`established-unauthenticated`); `handshake-unauthenticated` now also counts initiator drops. No reason renamed or removed |
| `ze_ipsec_error_notify_suppressed_total{reason}` | reader | `internal/le/interoplab/ipsec/unprotected_eap.go` `unprotDropCount` (embedded Python scrape of the Prometheus endpoint) | matches the exact series `reason="handshake-unauthenticated"` and asserts a rise by an exact amount | Preserved: its scenario has Ze as responder only, so initiator drops add nothing to its count; the new checker reuses the reader |
| `ze_ipsec_error_notify_suppressed_total{reason}` | reader | an operator's Prometheus, outside the repository | `docs/guide/ipsec.md`, "Health and metrics", documents the series by label only | Preserved: a new label value is additive; the guide gains the two reasons this spec touches |
| Doctor diagnostic, code `doctor-ipsec-unauthenticated-input` | writer | `ike/engine` new check, declared in `engine/register.go` `Registration.DoctorChecks` | sibling `checkIPsecVirtualIPPool` shows the shape | New: added with its code in `internal/core/diagnostic/codes.go` |
| Doctor diagnostic, code `doctor-ipsec-unauthenticated-input` | reader | `ze doctor` and every surface that renders `rpc.DoctorCheckDiagnostic` | renders any declared code; the implementer confirms no code list elsewhere needs the name (Architectural Verification, inbound row) | New: no reader edit expected |

### Integration Points
- `handleInbound`, `handleResponderInbound`, `handleOwnedInbound`, `handleHeldSuccessorInbound` - their handlers call the entry in place of the two functions it replaces.
- `countErrorNotifySuppressed` - the entry is the caller for every unauthenticated drop of an SK-protected message.
- `Registration.DoctorChecks` - one more declared check.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every SK-protected message still enters through `dispatchInbound` or `dispatchNATTInbound` (token bucket, SPI lookup), then `handleInbound` or `handleOwnedInbound`, and every handler opens it through `openProtected` (`protected_input.go`), the one caller of `decryptSKPayload` in non-test files, pinned by `TestOneFunctionOpensEveryProtectedMessage`. The doctor check reads the running SA table through `ActiveTable()`, as `checkIPsecVirtualIPPool` does |
| No unintended coupling (components stay isolated) | Yes | Phases 1-4 edit `ike/engine` and add one constant and one metadata entry to `internal/core/diagnostic/codes.go`, which imports nothing new. No import between components was added; the check lives in `ike/engine` and reaches the hub doctor through its `Registration.DoctorChecks` entry, so a forked `ike` plugin answers through the doctor callback |
| No duplicated functionality (extends existing, does not recreate) | Yes | `openProtected` replaced `decryptAndParse` and `discardUnauthenticatedHandshakeRequest`, both deleted (phases 2, 3); drops are counted through the existing `countErrorNotifySuppressed`; the doctor check is one more `registry.DoctorCheckDef` beside its nine siblings |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `openProtected` takes the parsed message and the raw octets the dispatcher already holds and decrypts them as `decryptAndParse` did; the drop adds one atomic add on the SA and allocates nothing that outlives the call |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The check is declared in the `ike` plugin's own `Registration.DoctorChecks` (`engine/register.go`, order 739); the hub runs plugin checks through `runPluginRegistryChecks` and the doctor callback without naming them. The code is a constant with its builtin metadata, the shape of every `doctor-ipsec-*` code; no switch or field list in a shared package names it |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched the repository (vendor, tmp, bin and plan excluded) for the sibling names `ipsec-virtual-ip-pool` and `CodeDoctorIPsecVirtualIPPool`, to find every list a check name lands in. `ze show diagnostic <code>` resolves through `diagnostic.Lookup` over the builtin metadata table, which is the code's declaration; `TestIPsecDoctorCodesRegistered` derives the codes it looks up from `registry.PluginDoctorChecks()`; the doctor-coverage gate reads covered codes from the `DoctorCheckDef` literals in source; `TestDoctorDependencyInventory` counts runtime dependencies, not problem classes, so it gains no row; `docs/features/ai-first.md` names two example codes and lists none. The prose surfaces that describe the IPsec doctor checks, `docs/guide/ipsec.md`, `docs/architecture/ike/ipsec-10-cli-diag.md` and `features/ipsec-cli-and-diagnostics.md`, were edited. The new reason `established-unauthenticated` is published from the counter map by `publishErrorNotifyCounts`, which holds no list of reasons; the one Go reader of a reason, `unprotDropMetric` (`internal/le/interoplab/ipsec/unprotected_eap.go`), names `handshake-unauthenticated` only |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `runInitiator` keeps its schedule after a handler returns without writing state | Read: `runInitiator`, `awaitHandshake`, `handleInbound` | The drop would stall or spin the loop | `TestInitiatorKeepsRetransmittingAfterUnprotectedResponse` | confirmed by reading; TestInitiatorKeepsRetransmittingAfterUnprotectedResponse observed red (errSADead) before 9019fe4d21 and green after |
| A-2 | `eapMessageDiscarded` matches an error wrapped with `errInnerParse` | Read: it uses `errors.Is`, and `handleResponderEAP` already relies on it over `decryptAndParse` | An EAP Length discard on the initiator would end the SA | `TestEAPLengthDiscardKeepsInitiatorSA` (existing behaviour, new route) | confirmed: errInnerChain wraps with %w, errors.Is still reaches ErrEAPLengthExceedsData; TestEAPLengthDiscardKeepsInitiatorSA green |
| A-3 | No caller needs the unauthenticated error value itself | Read: all twelve rows of Current Behavior; each only tests nil, or `errInnerParse`, or logs the error | The entry's "no error value" shape would lose something a caller uses | The compile, once `decryptAndParse` is deleted | confirmed by reading |
| A-4 | An AEAD message has no authenticated-malformed decrypt failure of its own | Read: `OpenIKEAEAD` returns `ErrDecryptionFailed` only for a short input or a failed tag, and `decryptSKPayload` returns its plaintext as is | An authenticated AEAD message could be dropped as a forgery | `TestDecryptMarksAuthenticatedPaddingFailureAsInner` (existing) stays green; a reviewer re-reads `decryptSKPayload`'s AEAD arm | confirmed by reading |
| A-5 | Taking the first SK payload instead of the last changes nothing | `handleAuthResponse` and `handleEAPResponse` keep the last SK payload of the chain, `decryptAndParse` the first. Whether `Message.ReadFrom` admits two is not read | A response carrying two Encrypted payloads would be opened differently | Read `wire.Message.ReadFrom`; if it admits two, add a unit test that pins the first | broken: `wire.Message.ReadFrom` (`ike/wire/message.go`) admits two. It does not stop at an Encrypted payload: it takes the SK payload's Next Payload (the first inner type) as the next outer type and keeps walking while `off < end`, so a message whose Length runs past the first SK payload, with that Next Payload 46, parses a second `*wire.PayloadSK`. Nothing enforces RFC 7296 Section 3.14, "The Encrypted payload, if present in a message, MUST be the last payload in the message." The first and the last SK payload are therefore different payloads in such a message. openProtected takes the first (protectedPayload), pinned by TestProtectedPayloadTakesTheFirstEncryptedPayload |
| A-6 | strongSwan with `encap = yes` makes Ze, as initiator, send IKE_AUTH on UDP 4500, so a port match separates IKE_SA_INIT from IKE_AUTH | Not read at the producer on either side. Ze floats in `handleSAInitResponse` on a NAT verdict; strongSwan's forced encapsulation is in `ike_natd.c`, not fetched | The hold rule of the interop scenario cannot be written by port | Read `ike_natd.c` at the lab's strongSwan version and Ze's `floatToNATTPort` caller before writing the checker. Fallback: match the IKE exchange type octet with an iptables `u32` rule, after confirming the lab kernel carries the match | broken premise, resolved another way (phase 6): Ze sends IKE_AUTH from UDP 4500 without `encap`, through `mobikeAuthOffer` (RFC 4555 Section 3.3) when `childMigrationAvailable`, and charon `build_response` answers along the request's path. `encap = yes` is not used: it fakes NAT_DETECTION_SOURCE_IP (`ike_natd.c` `build_natd_payload`, `force_encap`) and would make Ze record `PeerBehindNAT`. The `u32` fallback is impossible, because Ze's kernel lacks `xt_u32`. The hold is an nftables rule on strongSwan's output hook that matches the exchange type by a raw payload offset and exempts the forgery's UDP length 108, because rawprobe's injected datagrams cross the same output hook (Design Insights) |
| A-7 | strongSwan answers a retransmitted IKE_AUTH request from its cache after it established the SA | Read: `process_message` (`task_manager_v2.c`), the branch that calls `send_packets` on `responding.packets` | After the hold lifts, Ze's retransmission would draw nothing | The interop scenario's green run | confirmed by reading strongSwan 6.1.0 `process_message`: `is_retransmit` answers ALREADY_DONE when the Message ID is `responding.mid - 1` and the request hash equals `responding.prev_hash`, logs "received retransmit of request with ID %d, retransmitting response" and calls `send_packets(responding.packets)`; Ze resends `LastSentMsg` byte-identical. Confirmed by run: `ipsec-auth-corrupt-response-strongswan` green twice (49s, 53s), the tunnel establishing on charon's cached answer after the hold lifted (4999a0a410) |
| A-8 | The interop lab runs the strongSwan line whose source was read (6.1.0) | Owner rule: labs run the latest stable release. The digest still names 5.9.14 | A quoted function may differ at the lab's version | Read the version the lab image builds, and re-read `parse_message` there if it differs | confirmed: the lab builds 6.1.0 (`STRONGSWAN_VERSION` in `Dockerfile.strongswan`, tarball sha256 verified); `parse_message` `case FAILED:` logs "integrity check failed" under `/* ignored */`, as quoted above |
| A-9 | A relay fixture between two Ze daemons on loopback reaches an established tunnel | `nattAddr` gives each `.ci` the port after `$PORT2` for NAT-T; both daemons see a NAT through a relay address and float there. Not run | The `.ci` needs another injection route | Draft under `test/draft/ipsec/` first; a control run with the relay forwarding untouched must establish | confirmed by reading (phase 5): the NAT_DETECTION hashes use the configured addresses with port 500, so both daemons mismatch on the relay address and float through `floatToNATTPort`; `remoteUDPAddr` dials `nattAddr`; the responder matches the peer by source IP and replies to the observed source. Confirmed by run: the passthrough control (`test/draft/ipsec/ike-auth-corrupt-response-control.ci`, relay argument `passthrough`) passed under `./le --name session test ipsec --draft`, after phases 1-4 landed (9019fe4d21, a49d4d301f) | confirmed |
| A-10 | A `.ci` can run a compiled fixture in the background beside two daemons | Existing `.ci` files run a fixture in the foreground only | The relay must be started another way | Read `docs/architecture/testing/ci-format.md` before drafting | confirmed by reading: `ci-format.md`, "ready= -- wait for a background process to serve", shows `cmd=background ... :ready=<text>`, and an `expect=stdout` under a background cmd reads its span. Confirmed by run: the relay runs as `cmd=background ... :ready=RELAY: listening on` beside both daemons, and the control and the corrupt-response drafts both passed | confirmed |
| A-11 | A drop counter on the SA, read by the doctor check from another goroutine, is safe as an atomic | `SA` fields are otherwise owned by one goroutine at a time | A data race under `-race` | The engine package tests under the race detector | confirmed by reading (phase 4): `SA.unauthenticatedDrops` is a plain `uint64` written only through `atomic.AddUint64` (`dropUnauthenticated`) and read only through `atomic.LoadUint64` (the doctor check), the idiom `hardExpiry` uses because the twin-SA tests copy the struct. The check also reads `SA.State` and `SA.PeerName` from `table.All()`, as `checkIPsecHealth` (`health.go`) and `IPsecMetrics.Update` (`metrics.go`) already do; that plain read of `State` predates this spec and belongs to `spec-validated-construction-and-state-types.md` (D3). Confirmed by run: the engine package ran under the race detector with the doctor tests green; its one red, `TestInterfaceAbsenceLoggedOncePerAbsence` (`dynamic_wan_test.go`), races in the full package run and passes alone, is not caused by this spec, and is journaled in `plan/journal/test-reads-producer-state-unlocked.md` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A persistent forger keeps the initiator from ever seeing the genuine response inside one retransmit window | The handshake ends with `errTimeout` and the drop counter rises | Not a regression: today the first forgery ends it. The token bucket bounds the rate, the genuine response is processed whenever it arrives, and the doctor check names the cause |
| R-2 | Each forged response wakes `runInitiator` for one pass | CPU on the peer session goroutine under a flood | One pass reads two fields and waits again; the inbound token bucket bounds it at its burst and rate |
| R-3 | Discrimination records fingerprinted on the edited producers go stale | `./le rfc check` reports producer-changed records for `handleAuthRequest`, `handleResponderEAP`, `handleResponderAuthRound`, `handleOwnedInbound`, `handleHeldSuccessorInbound`, `handleAuthResponse`, `handleEAPResponse` | Re-record each in the phase that edits its producer, and name any left red in the commit body with its cause |
| R-4 | `plan/release/fixes/spec-ike-responder-auth-malformed-request-notify.md` edits the same three responder branches | A conflict in `responder.go`, `responder_eap.go`, `multiple_auth.go` | This spec leaves the authenticated-malformed branch of those handlers as it finds it. Whichever lands second rebases on the first |
| R-5 | Moving a site onto the entry changes its reaction to authenticated malformed content by accident, most easily in the second exchange, where the decrypt step and the chain parse react differently | A red in `TestInitiatorSecondAuthExchangeKeepsSAOnAuthenticatedPaddingFailure` or in an existing established-SA INVALID_SYNTAX test | The per-site table in Current Behavior is the preservation list; the entry reports which step failed |
| R-6 | The new established-site counts change what an existing test asserts about a reason | A red in `rfc7296_invalidmsgid_test.go` or in a test asserting no rise | The two existing reasons keep their sites; the new reason is used only where nothing is counted today |
| R-7 | The doctor check stays raised after the cause is gone | A warning on a healthy box | The count lives on the SA of the current handshake, and the check reports only a peer whose SA is not established, so an established tunnel or a new cycle clears it |
| R-8 | The debug line of the drop is the only log, and Ze logs at WARN by default | An operator sees a tunnel that does not come up and no line | That is the decided trade (a WARN per forgery lets an off-path sender write the log). The doctor check and the counter are the operator's record |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Site-to-site tunnels. A wrong classification either ends handshakes on forgeries (today's defect) or keeps an SA alive on a message that should end it; a wrong edit to `handleOwnedInbound` reaches every established tunnel |
| How is it reverted? | Each phase is one commit and reverts alone. Nothing a peer has seen needs migration |
| Who else touches this path? | `spec-ike-responder-auth-malformed-request-notify.md` (the responder's authenticated-malformed branch), `spec-ike-eap-one-off-defects.md` (`handleCreateChildSAOwned`, `eap/peer.go`), and the post-release `spec-validated-construction-and-state-types.md` (every `sa.State` write) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH response with a failed checksum, delivered through `handleInbound` to an SA in `StateAuthSent` | → | the entry, called by `handleAuthResponse` | `TestInitiatorAuthSurvivesUnprotectedIKEAuthResponse` |
| IKE_AUTH response with a failed checksum, delivered through `handleInbound` to an SA in `StateEAPInProgress` | → | the entry, called by `handleEAPResponse` | `TestInitiatorEAPSurvivesUnprotectedIKEAuthResponse` |
| A datagram on the daemon's IKE socket during a real handshake | → | `dispatchInbound`, `handleInbound`, the entry | `test/ipsec/ike-auth-corrupt-response.ci` |
| A forged IKE_AUTH response sent to Ze by a lab peer | → | the same path, against strongSwan | interop scenario `ipsec-auth-corrupt-response-strongswan`, checker `checkInitiatorSurvivesUnprotectedAuthResponse` (`internal/le/interoplab/ipsec/unprotected_response.go`) |
| `ze doctor` on a daemon whose handshake dropped forgeries | → | the check `ipsec-unauthenticated-input`, `checkIPsecUnauthenticatedInput` (`doctor_unauthenticated_input.go`) | `TestDoctorReportsHandshakeReceivingUnprotectedMessages` |
| `ze doctor` on a daemon whose established SA received authenticated messages that do not parse | → | the check `ipsec-peer-malformed-message`, `checkIPsecPeerMalformedMessage` (`doctor_peer_malformed.go`) | `TestDoctorReportsPeerSendingMalformedMessages` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Initiator in `StateAuthSent`; an IKE_AUTH response under the live SPIs whose checksum or AEAD tag fails arrives, then the peer's genuine response | After the forgery the SA is still `StateAuthSent` with the same next Message ID, retransmit time and retransmit count, no datagram was sent, and `handshake-unauthenticated` rose by one. The genuine response then establishes the same SA, with the same SPI pair | A response that verifies and then fails still ends the handshake: a failed AUTH, an unrecognized error notify, a prohibited NAT. They are authenticated, so RFC 7296 Section 2.4 does not cover them |
| AC-2 | Initiator in `StateEAPInProgress`; the same forgery, then the authenticator's genuine next round | The SA stays `StateEAPInProgress` at the same Message ID and the counter rose by one; the genuine round continues the exchange to an established SA | An EAP method failure and a missing EAP payload in an authenticated message still end the handshake |
| AC-3 | Initiator in the RFC 4739 second exchange; the same forgery | The SA is kept, as today, and the drop is now counted under `handshake-unauthenticated` and logged at debug | A response at another Message ID is still dropped before any decrypt and is not counted: it is the existing guard against a late copy of the first response |
| AC-4 | Initiator, first exchange or an EAP round; a response whose checksum verifies and whose inner chain or CBC padding does not parse | The site keeps what it does today: the SA goes to `StateDead`, `runInitiator` returns `errSADead`, the counter does not move, and nothing is sent | An authenticated message whose only defect is an EAP Length larger than its octets keeps the SA (RFC 3748 Section 4, `eapMessageDiscarded`). In the RFC 4739 second exchange a CBC padding failure keeps the SA and a chain that does not parse ends it, both as today: no RFC sentence decides between them (Design Insights) |
| AC-5 | Initiator; an IKE_AUTH response with no Encrypted payload | Dropped with the SA unchanged, as today, and now counted under `handshake-unauthenticated` | None: nothing in such a message is authenticated |
| AC-6 | Initiator handshake; only forgeries arrive | `runInitiator` resends the identical request octets at each scheduled retransmit time and returns `errTimeout` after `maxRetransmissions`, the same count and schedule as with no input at all | The reconnect that follows `errTimeout` is the ordinary one: RFC 7296 Section 2.4 lets an endpoint conclude failure "only when repeated attempts to contact it have gone unanswered for a timeout period" |
| AC-7 | Initiator in `StateAuthSent`; a forgery arrives from another source address and port | The SA's stored peer endpoint is unchanged; the genuine response then sets it to the address it came from | The endpoint follows an authenticated message only, which is what `handleInbound` already requires |
| AC-8 | Responder handshake at its three sites; a forged request | Behaviour is byte-for-byte what it is today: dropped, counted under `handshake-unauthenticated`, SA untouched, and the peer's genuine request completes the exchange | An authenticated malformed request still ends the handshake with nothing sent; the notify it is owed belongs to `spec-ike-responder-auth-malformed-request-notify.md` |
| AC-9 | Established SA; an in-window message, an out-of-window INFORMATIONAL response and, on a MOBIKE SA, a message at the cached Message ID, each with a failed checksum | The SA stays established with its Child SA, no liveness is credited, nothing is sent, and `established-unauthenticated` rose by one for each; a genuine INFORMATIONAL request is then answered | An authenticated malformed request still draws INVALID_SYNTAX or UNSUPPORTED_CRITICAL_PAYLOAD, and an out-of-window request that fails its checksum is still counted under `invalid-msgid-unauthenticated` |
| AC-10 | The engine source after the change | Every handler that reads an SK-protected message obtains the chain from the entry: `decryptSKPayload` has one caller in non-test engine files, and `decryptAndParse` and `discardUnauthenticatedHandshakeRequest` no longer exist | `decryptSKPayload` remains as the cryptographic step, and a test may call it directly to pin the `errInnerParse` mark |
| AC-11 | `ze doctor` while a configured peer's handshake has dropped at least one message that failed its integrity check and has not established | One warning with code `doctor-ipsec-unauthenticated-input` that names the peer, the number dropped and the handshake state, says the messages failed their integrity check so Ze kept waiting for the peer's answer, and gives the command that shows the counter and the next step for the operator | A peer that established, and a peer with no drop, are not reported: a forgery that changed nothing is not a problem the operator has |
| AC-12 | Ze initiates to strongSwan; forged IKE_AUTH responses reach Ze before the genuine one | Ze's counter rises by exactly the number injected, and the tunnel establishes on the IKE SA whose initiator SPI was forged against, and passes traffic | strongSwan's own SA may establish before Ze's: it authenticated Ze's request and cannot know its answer was held |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up a site-to-site tunnel that Ze initiates while a third party sends IKE_AUTH responses under the SPIs it observed | UDP socket -> `dispatchInbound` -> `handleInbound` -> `handleAuthResponse` -> the entry drops -> `runInitiator` retransmits -> genuine response -> established | `test/ipsec/ike-auth-corrupt-response.ci`, interop `ipsec-auth-corrupt-response-strongswan` |
| 2 | Runs `ze doctor` because a tunnel is slow to come up | hub doctor -> `ike` plugin check -> the SA's drop count | `TestDoctorReportsHandshakeReceivingUnprotectedMessages` |
| 3 | Watches an established tunnel while garbage arrives for it | owner loop -> `handleOwnedInbound` -> the entry drops and counts | `TestEstablishedSACountsUnprotectedMessage` |

## 🧪 TDD Test Plan

### Unit Tests

New tests go in `internal/component/ike/engine/rfc7296_unprotected_response_test.go`
unless a row names another file. The handshake is driven as
`rfc7296_unprotected_handshake_test.go` drives it: two in-process SAs, each
side's last sent message delivered to the other through `handleInbound`. A
forgery is the genuine message with its last octet changed (a failed checksum)
or with its Encrypted payload removed. An authenticated malformed message is
sealed with the SA's own keys over a chain that does not parse.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInitiatorAuthSurvivesUnprotectedIKEAuthResponse` | as above | AC-1. Tag `RFC7296-2.4-3 negative` | |
| `TestInitiatorEAPSurvivesUnprotectedIKEAuthResponse` | as above | AC-2. Tag `RFC7296-2.4-3 negative` | |
| `TestInitiatorSecondAuthExchangeCountsUnprotectedResponse` | as above | AC-3 | |
| `TestInitiatorAuthEndsOnAuthenticatedMalformedResponse` | as above | AC-4, first exchange. Tag `RFC7296-2.4-3 positive` | |
| `TestInitiatorEAPEndsOnAuthenticatedMalformedResponse` | as above | AC-4, EAP round | |
| `TestInitiatorSecondAuthExchangeKeepsSAOnAuthenticatedPaddingFailure` | as above | AC-4 survivor: today's second-exchange reaction to a CBC padding failure, preserved through the entry | |
| `TestEAPLengthDiscardKeepsInitiatorSA` | as above | AC-4 survivor: the discard still keeps the SA through the entry | |
| `TestInitiatorAuthResponseWithoutSKIsCounted` | as above | AC-5 | |
| `TestInitiatorKeepsRetransmittingAfterUnprotectedResponse` | as above | AC-6: after a forgery, the next send is the same octets at the unchanged retransmit time. Uses the `afterFunc` seam | |
| `TestInitiatorTimesOutWhenOnlyUnprotectedResponsesArrive` | as above | AC-6: `errTimeout` after `maxRetransmissions` | |
| `TestUnprotectedAuthResponseDoesNotMoveThePeerEndpoint` | as above | AC-7 | |
| `TestEstablishedSACountsUnprotectedMessage` | as above | AC-9, in-window message | |
| `TestEstablishedSACountsUnprotectedOutOfWindowResponse` | as above | AC-9, out-of-window INFORMATIONAL response | |
| `TestMobikeRetransmitArmCountsUnprotectedMessage` | `internal/component/ike/engine/mobike_test.go` | AC-9, the MOBIKE retransmit arm | |
| `TestOneFunctionOpensEveryProtectedMessage` | `internal/component/ike/engine/protected_input_test.go` | AC-10: parses the package's non-test files and finds one call expression naming `decryptSKPayload`. It is decidable from syntax because it pins the callers of one named package-level function. It cannot see a function value or a method expression that aliases the opener, so it proves no more than that no handler calls the primitive by name | |
| `TestDoctorReportsHandshakeReceivingUnprotectedMessages` | `internal/component/ike/engine/doctor_unauthenticated_input_test.go` | AC-11, the reported case | |
| `TestDoctorIsSilentOnceThePeerEstablished` | same file | AC-11, the survivor | |
| The seven tests of `rfc7296_unprotected_handshake_test.go`, `TestErrUnprotectedNotifyLeavesHalfOpenSA`, the tests of `rfc7296_invalidmsgid_test.go` | existing | AC-8 and the preserved reasons: run unedited, before and after each phase | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Retransmit count before `errTimeout` | 0 to `maxRetransmissions` | `maxRetransmissions` | N/A | one more send would be a defect; `TestInitiatorTimesOutWhenOnlyUnprotectedResponsesArrive` counts the sends |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ike-auth-corrupt-response` | `test/ipsec/ike-auth-corrupt-response.ci` | Two Ze daemons negotiate through a relay. Before the relay forwards each first IKE_AUTH response to the initiator, it sends three copies with the last octet changed. The operator's tunnel still establishes | green (1.6s); promoted to `test/ipsec/`, tagged `RFC7296-2.4-3 negative`; red recorded under the hand-written mutant of `openProtected` that returns the failed integrity check as an error (no `sa-up` within 30s); committed in 78874013b5 with `rfc/discrimination/rfc7296/RFC7296-2.4-3.json` |

How it is built. A new compiled fixture, `ipsec/auth-corrupt-response`, binds
a third loopback address on the test's IKE port and on the NAT-T port after
it, and forwards datagrams between the initiator and the responder, each of
which is configured with the relay's address as its remote. It recognises an
IKE_AUTH response by the exchange type and the response flag of the header in
the clear (after the non-ESP marker on the NAT-T port). It returns an error if
it never saw one. The `.ci` asserts three things: the establishment gate the
other `ipsec` tests use (`show vpn ipsec sa`), the initiator's production
debug line for the drop (with the `ike` log subsystem raised to debug), and
the absence of the SA-dead exit. Its red phase: with the fix reverted the
first copy sets `StateDead` in every cycle, so the tunnel never establishes
and the drop line never appears.

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ipsec-auth-corrupt-response-strongswan` | `test/interop-ipsec/scenarios/` | strongSwan | Ze as initiator keeps its handshake through forged IKE_AUTH responses and completes it on strongSwan's genuine, retransmitted answer | green (49s and 53s); tagged `RFC7296-2.4-3 negative`; red recorded under the hand-written `openProtected` mutant at assertion 5 (count rose by 1, want 20); committed in 4999a0a410. Assertions are numbered (count = 5, SPI = 7) so the record can cite them |

Method, mirroring `checkResponderEAPSurvivesUnprotectedAuth`:

1. Control: the tunnel establishes once, unmolested. Its SPI pair is recorded, and it is terminated only after step 2 installs the hold: Ze reconnects about one second after strongSwan's Delete, and the Delete (exchange 37) passes the hold.
2. Hold: an nftables table (`inet ze_auth_hold`) on the output hook of the strongSwan container drops its own datagrams to Ze from UDP 4500 whose exchange type octet, matched by raw payload offset, is IKE_AUTH (35), except a datagram of UDP length 108, the forgery's: rawprobe's injected datagrams cross the same output hook. IKE_SA_INIT and INFORMATIONAL still pass. Ze sends IKE_AUTH from 4500 without `encap` (A-6).
3. Ze initiates. strongSwan authenticates Ze and answers into the hold. Ze waits in `StateAuthSent` and retransmits. The checker waits for strongSwan's log to show it received a retransmitted request.
4. The checker reads the SPI pair from `swanctl --list-sas` and records the initiator SPI.
5. It injects twenty forged IKE_AUTH responses from strongSwan's address and port through `injectOutOfSA`: the live SPI pair, the Response flag set and the Initiator flag clear, Message ID 1, and an Encrypted payload of filler octets that fails its integrity check.
6. Ze's `handshake-unauthenticated` series must rise by exactly twenty.
7. The hold is lifted. Ze's next retransmission draws strongSwan's cached response. Establishment is read on Ze's side, from `show vpn ipsec sa | json`, because strongSwan lists its SA as ESTABLISHED once it has sent its response, held or not: Ze's established initiator SA must carry the recorded initiator SPI, and the tunnel must pass traffic.

Steps 2 to 7 must fit inside Ze's retransmit window, which follows from
`retransmitBase`, `retransmitBackoff` and `maxRetransmissions`; the checker
names the bound it computed.

What the test expects of the peer, and where that was read (strongSwan 6.1.0):
`process_message` (`task_manager_v2.c`) answers a request at the previous
Message ID from `responding.packets`. For reference, strongSwan in Ze's
position does what this spec makes Ze do: `parse_message` in the same file has
`case FAILED:` logging "integrity check failed" with the comment "ignored",
leaves the status at `FAILED`, and for an SA in `IKE_CONNECTING` raises no
event; `execute` (`process_message_job.c`) destroys the IKE SA only when
`process_message` answers `DESTROY_ME`.

Red phase: revert the initiator phase, rebuild the Ze image, and run. The
counter does not rise and the SA that establishes carries a new initiator SPI,
so steps 6 and 7 both fail. The two tagged unit tests and this scenario are
recorded with `./le rfc record`.

## Files to Modify
- `internal/component/ike/engine/fsm.go` - `handleAuthResponse` and `handleEAPResponse` open the message through the entry; their SK-payload scan and the missing-SK drop go; the second exchange keeps its reaction to a padding failure
- `internal/component/ike/engine/inbound.go` - `decryptAndParse` is deleted; `handleOwnedInbound`'s four sites call the entry; `errInnerParse` stays
- `internal/component/ike/engine/responder.go`, `responder_eap.go`, `multiple_auth.go` - the three responder sites call the entry
- `internal/component/ike/engine/held_successor.go` - its site calls the entry
- `internal/component/ike/engine/auth.go` - `decryptSKPayload`'s comment names its one caller
- `internal/component/ike/engine/sa.go` - the drop count of the SA's handshake
- `internal/component/ike/engine/register.go` - the doctor check declaration
- `internal/core/diagnostic/codes.go` - `doctor-ipsec-unauthenticated-input`
- `internal/le/interoplab/ipsec/checkers.go` - the scenario's row
- `ai/doors/access-security.md` - the trap paragraph
- `ai/digests/ipsec-ike.md` - flow items 9, 10 and 15, and the invariant on established-SA inbound
- `docs/architecture/ike/ipsec-14-responder.md`, `ipsec-9-ikev2-eap-nat.md`, `ipsec-7-ikev2-engine.md` - the paragraphs named in Required Reading
- `docs/architecture/ike/ipsec-10-cli-diag.md` - the doctor check
- `docs/guide/ipsec.md` - "Health and metrics": the reasons, and the doctor warning
- `docs/architecture/testing/interop.md` - the scenario, if the page lists IPsec scenarios
- `features/ikev2-engine.md` - the Interop row
- `rfc/short/rfc7296.md` - row `RFC7296-2.4-3`, after the independent audit
- `plan/journal/unprotected-message-changes-sa-state.md` - the 2026-09-29 row's Fix cell

## Files to Create
- `internal/component/ike/engine/protected_input.go` - the entry and the reason names; replaced `responder_unauthenticated.go`, deleted in 9019fe4d21
- `internal/component/ike/engine/protected_input_test.go`
- `internal/component/ike/engine/rfc7296_unprotected_response_test.go`
- `internal/component/ike/engine/doctor_unauthenticated_input.go`, `doctor_unauthenticated_input_test.go`
- `internal/component/ike/engine/doctor_peer_malformed.go`, `doctor_peer_malformed_test.go` - the second problem class phase 3's WARN lines raised (Design Insights)
- `internal/test/fixture/tunnel_fixture_ipsec_corrupt_response.go` - the relay fixture
- `internal/test/fixture/register_ipsec_auth_corrupt_response.go` - the relay's registration: `writeGoPatterns` requires an `init()` that registers to live in a `register*.go` file
- `test/ipsec/ike-auth-corrupt-response.ci`
- `test/interop-ipsec/scenarios/ipsec-auth-corrupt-response-strongswan/` - `ze.conf`, `swanctl.conf`
- `internal/le/interoplab/ipsec/unprotected_response.go` - the checker
- `internal/le/interoplab/ipsec/unprotected_response_test.go` - the checker's unit tests: the retransmit schedule copy against `fsm.go`, the window bounds, the forged datagram's shape and the nft offset, the SPI pair pick, the read of Ze's JSON
- `rfc/discrimination/` records for the tagged units, written by `./le rfc record`
- `website/changes/fragments/` - one fragment, as the responder fix carried

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config leaf and no RPC: the behaviour is not optional |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command added; `ze doctor` gains a check through registration |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC; the behaviour's functional test is `test/ipsec/ike-auth-corrupt-response.ci` |
| Pipe completeness | N-A | No command output added |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file, socket, port, module or binary |
| Doctor code per problem class | Yes | Problem class: a handshake in progress that receives messages failing their integrity check. Check in `ike/engine/doctor_unauthenticated_input.go`, declared in `Registration.DoctorChecks` as `ipsec-unauthenticated-input` at phase `rpc.DoctorPhasePostConfig` with the next free order after its siblings, code `doctor-ipsec-unauthenticated-input` in `internal/core/diagnostic/codes.go`, severity warning, read over the running engine like `checkIPsecVirtualIPPool`. The two WARN lines this change removes ("AUTH response decryption failed", "EAP response decryption failed") had no code; the authenticated-malformed WARN lines stay as they are |
| Prometheus counters/metrics | Yes | `ze_ipsec_error_notify_suppressed_total{reason}`: `handshake-unauthenticated` now counts both roles; new reason `established-unauthenticated`; no new series |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/ikev2-engine.md`: the Interop row gains the scenario |
| 2 | Config syntax changed? | No | No leaf |
| 3 | CLI command added/changed? | No | No command |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | The `ike` plugin's registration gains a doctor check, which `docs/guide/plugins.md` does not list per check; the implementer confirms by grep |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`, "Health and metrics" |
| 7 | Wire format changed? | No | No byte changes (Changed-Wire Census) |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7296.md` row `RFC7296-2.4-3`: the `{partial}` annotation and its owner pointer, re-judged by an independent `ze-rfc-audit`; `rfc/audit/rfc7296/RFC7296-2.4-3.json` follows the audit |
| 10 | Test infrastructure changed? | Yes | A new fixture: `docs/functional-tests.md` if it lists fixtures; `docs/architecture/testing/interop.md` for the scenario |
| 11 | Affects daemon comparison? | No | No comparison row names this behaviour; confirm by grep of `docs/comparison.md` for "integrity" and "forged" |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-14-responder.md` (the drop now serves both roles and names the entry), `ipsec-9-ikev2-eap-nat.md` (the unprotected-response paragraph), `ipsec-7-ikev2-engine.md` (the second-exchange sentence), `ai/digests/ipsec-ike.md`, `ai/doors/access-security.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/ipsec.md`; `docs/plugin-development/metrics.md` only if it lists the reasons, confirmed by grep |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The doctor check inventory: `docs/architecture/ike/ipsec-10-cli-diag.md`, and any generated doctor inventory the tooling names |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived: run `./le spec citation anchors spec plan/release/fixes/spec-single-owner-lifecycle-4-ike-unauthenticated-input.md`. Known from reading: the anchors on `responder_unauthenticated.go` in `ipsec-14-responder.md` move to `protected_input.go`. Two pages are declared by a `// Design:` header of a changed file and are not otherwise edited: `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (declared by `inbound.go`) is unaffected, because this change alters no Child SA, rekey or XFRM behaviour and the page's account of `handleOwnedInbound` is checked against the moved call in phase 3; `docs/features/ai-first.md` (declared by `internal/core/diagnostic/codes.go`) describes `ze show diagnostic <diagnostic-code>`, so phase 4 confirms the new code answers there and edits the page only if it lists codes |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No example shows this path |

## Implementation Steps

Each phase is one commit that compiles, passes its tests and improves the
daemon on its own. Pages are edited in the phase that makes them wrong.

1. **Phase: Wiring (MANDATORY FIRST)** -- the failing tests that prove the defect through the real handler path
   - Tests: `TestInitiatorAuthSurvivesUnprotectedIKEAuthResponse`, `TestInitiatorEAPSurvivesUnprotectedIKEAuthResponse`, `TestInitiatorKeepsRetransmittingAfterUnprotectedResponse`
   - Files: `rfc7296_unprotected_response_test.go`
   - Verify: the first two fail on `StateDead`; the third fails on `errSADead`. They are committed with phase 2, not alone
2. **Phase: the entry, and the initiator through it** -- add the entry in `protected_input.go` with the three outcomes; move the three responder handshake sites and the two initiator handlers onto it; delete `discardUnauthenticatedHandshakeRequest` and its file
   - Tests: every initiator row of the Unit Tests table; the existing responder tests unedited
   - Files: `protected_input.go`, `fsm.go`, `responder.go`, `responder_eap.go`, `multiple_auth.go`; pages: `ai/doors/access-security.md`, `ipsec-14-responder.md`, `ipsec-9-ikev2-eap-nat.md`, `ipsec-7-ikev2-engine.md`, digest items 9 and 10; the journal row's Fix cell; the website fragment
   - Verify: tests fail → implement → tests pass; `./le test unit pkg path ./internal/component/ike/engine/`; discrimination records for the two negative tags and the positive tag, and re-records for the producers this phase edited
3. **Phase: the established sites through the entry** -- move `handleOwnedInbound`'s four sites and the held successor's site; delete `decryptAndParse`; add the reason `established-unauthenticated`
   - Tests: `TestEstablishedSACountsUnprotectedMessage`, `TestEstablishedSACountsUnprotectedOutOfWindowResponse`, `TestMobikeRetransmitArmCountsUnprotectedMessage`, `TestOneFunctionOpensEveryProtectedMessage`; `rfc7296_invalidmsgid_test.go` unedited
   - Files: `inbound.go`, `held_successor.go`, `auth.go`; pages: digest item 15 and its invariant, `docs/guide/ipsec.md` reasons
   - Verify: as phase 2, plus re-records for `handleOwnedInbound` and `handleHeldSuccessorInbound`
4. **Phase: doctor** -- the drop count on the SA, the check, its code
   - Tests: `TestDoctorReportsHandshakeReceivingUnprotectedMessages`, `TestDoctorIsSilentOnceThePeerEstablished`
   - Files: `sa.go`, `protected_input.go`, `doctor_unauthenticated_input.go`, `register.go`, `codes.go`; pages: `ipsec-10-cli-diag.md`, `docs/guide/ipsec.md`
   - Verify: read `docs/architecture/doctor-and-health-checks.md` and `ai/rules/user-facing-errors.md` first; the engine package tests under the race detector
5. **Phase: functional test** -- the relay fixture and the `.ci`
   - Tests: `test/ipsec/ike-auth-corrupt-response.ci`
   - Files: `tunnel_fixture_ipsec_corrupt_response.go`, the `.ci`, drafted under `test/draft/ipsec/` and promoted
   - Verify: a control run with the relay forwarding untouched establishes (A-9); red with phase 2 reverted in the built daemon; green restored; `./le test functional ipsec` for the one test by name
   - Status: committed in 78874013b5. The control and the corrupt-response drafts passed, and the `.ci` is promoted to `test/ipsec/` with its `RFC7296-2.4-3 negative` tag. Red recorded through the hand-written mutant that turns the drop return of `openProtected` (`protected_input.go`) from `return nil, true, nil` into `return nil, false, err`: green in 1.6s, no `sa-up` within 30s under the break. The control stays an untracked draft
6. **Phase: interop** -- the scenario, its checker, its records, the audit
   - Tests: `ipsec-auth-corrupt-response-strongswan`
   - Files: the scenario directory, `unprotected_response.go`, `checkers.go`, `features/ikev2-engine.md`, `interop.md`
   - Verify: resolve A-6 and A-8 by reading before writing the checker; green run, reverted-red run with the image rebuilt, both recorded; then an independent `ze-rfc-audit` re-judges `RFC7296-2.4-3`, and its verdict edits `rfc/short/rfc7296.md`
   - Status: committed in 4999a0a410. Green (49s, 53s); red recorded under the hand-written `openProtected` mutant (the failed integrity check returned as an error), failing at assertion 5 with the count risen by 1, want 20. Two checker defects were found and fixed on the way (Design Insights). Owed: the independent `ze-rfc-audit` of `RFC7296-2.4-3`
- Commits: 9019fe4d21 (phases 1-2), a49d4d301f (phases 3-4), 78874013b5 (phase 5), 4999a0a410 (phase 6). Known red not caused by this spec: `TestInterfaceAbsenceLoggedOncePerAbsence` (`dynamic_wan_test.go`) races under `-race` in the full engine package run and passes alone; journaled in `plan/journal/test-reads-producer-state-unlocked.md`, not yet owned by a spec, because `spec-ike-eap-one-off-defects.md`, the spec meant to own it, refuses edits until it gains an Existing Mechanisms section

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named file and symbol |
| Feature completeness | The three user stories each have a passing test |
| Correctness | The entry writes no field of the SA on a drop except the drop count; the drop sends nothing; an `errInnerParse` error never reaches the drop branch; `eapMessageDiscarded` is still asked first at the three EAP-carrying handlers |
| Correctness, no layering | `decryptAndParse` and `discardUnauthenticatedHandshakeRequest` are gone, not wrapped; no handler scans for the SK payload itself |
| Naming | The entry's name, its log text and the comment on `handshake-unauthenticated` are true for a request and a response, a handshake and an established SA; reason values stay kebab-case |
| Data flow | Classification happens in the entry only; handlers do not test an error for "was it authenticated" except through `errInnerParse` on the entry's error |
| Stale comments | The comments in `handleAuthResponse` and `handleEAPResponse` that cite Section 2.21.4 for the missing-SK drop, and the constant's comment that says "IKE_AUTH request", match the new code |
| Operator errors and doctor codes | The check's message names the peer, the count, why Ze kept waiting, and the command; its code is in `codes.go`; the removed WARN lines leave no WARN without a code behind |
| Rule: `ai/rules/rfc-compliance.md` | The Section 2.4 sentence is quoted byte-exact above the drop, full stop inside the quote; each caller carries `// RFC 7296 Section 2.4` at the call |
| Rule: `ai/rules/testing.md` | No existing tagged test is edited; each new test asserts an exact outcome and a rise by an exact amount, never a non-zero absolute |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| One caller of `decryptSKPayload` in non-test engine files | a grep for the call in `internal/component/ike/engine`, test files excluded, shows the definition and one call |
| `decryptAndParse`, `discardUnauthenticatedHandshakeRequest` gone | the same grep for each name returns nothing |
| Initiator keeps its handshake | the unit tests of phase 2, output pasted |
| `.ci` and interop scenario exist and discriminate | their green run and their recorded red run |
| Doctor code declared and reachable | a grep for `doctor-ipsec-unauthenticated-input` in `internal/core/diagnostic/codes.go` and `internal/component/ike/engine` |
| Door, digest and pages true | `./le doc check all` |
| Journal row closed | the 2026-09-29 row's Fix cell reads `fixed:` with the date, the function and the proof |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The classification is made on the cryptographic result only, never on a field of the cleartext header |
| Amplification | The drop path sends nothing in any role or state |
| Log injection | No line above debug is written per unauthenticated datagram |
| Resource exhaustion | The drop allocates nothing that outlives the call; the doctor's count is one integer per SA; the token bucket still precedes the entry |
| Fail-open | A message is treated as authenticated only after `decryptSKPayload` returned without error or with an `errInnerParse` mark; an unknown error is a drop, never an opened message |
| State oracle | The drop is indistinguishable on the wire from a message for an SA Ze does not hold |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| An existing tagged test goes red | The code is wrong. Never edit the test; a change to it needs an independent `ze-test-judge` |
| A-6 or A-9 does not hold | Use the fallback its row names, and record the route taken in Design Insights |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- What a site does with authenticated malformed content is not uniform today, and this spec leaves it as it is (main-thread ruling, 2026-11-11: the spec changes the reaction to unauthenticated input only). Observed: the first exchange and each EAP round end the handshake; in the RFC 4739 second exchange `handleAuthResponse` keeps the SA on a CBC padding failure and ends it on a chain that does not parse; on an established SA a malformed request draws INVALID_SYNTAX, after which `respondError` sets `StateDead`, or UNSUPPORTED_CRITICAL_PAYLOAD, which does not; a malformed response on an established SA is dropped.
- The RFC text does not decide the second-exchange difference. Sentences read: RFC 7296 Section 2.21, "Such errors in parsing or processing response packets should still cause the recipient to clean up the IKE state (for example, by sending a Delete for a bad SA)." It leans toward ending the SA and carries no RFC 2119 keyword. Section 2.21.2, "Note, however, that request messages that contain an unsupported critical payload, or where the whole message is malformed (rather than just bad payload contents), MUST be rejected in their entirety, and MUST only lead to an UNSUPPORTED_CRITICAL_PAYLOAD or INVALID_SYNTAX Notification sent as a response." It binds the receiver of a request, not of a response. Section 2.21.3, "If a peer parsing a request notices that it is badly formatted (after it has passed the message authentication code checks and window checks) and it returns an INVALID_SYNTAX notification, then this error notification is considered fatal in both peers, meaning that the IKE SA is deleted without needing an explicit Delete payload." It is what the established-SA path does. RFC 4739 states nothing on a malformed message. No journal row was added.
- "The function consumes the failure, so a caller never holds an error" needs a reason from each caller: the established sites already count under two different reasons, and one of them (`invalid-msgid-unauthenticated`) is asserted by tests.
- An established SA already survives garbage at all five sites. What was missing there is the count, at three of them.
- The interop hold took another route than A-6 planned (Failure Routing, "A-6 or A-9 does not hold"). The premise was wrong: Ze needs no `encap = yes` to send IKE_AUTH from UDP 4500, because `mobikeAuthOffer` floats it there, and `encap = yes` would have made Ze record the peer behind a NAT. The planned fallback, an iptables `u32` match, is impossible on Ze's kernel, which does not build `xt_u32`. The hold is therefore an nftables raw payload match on the exchange type in strongSwan's output hook. A hold by port or by exchange type also drops the forgeries, since rawprobe's raw socket passes the same output hook as charon, so the rule exempts the forgery's UDP length (108). The control SA is terminated after the hold is in place, because Ze reconnects about a second after the peer's Delete. Ze's side is the establishment check (`show vpn ipsec sa | json`), because strongSwan as responder lists ESTABLISHED once it has sent its response.
- Phase 3: about a hundred test call sites, `rfc7296_invalidmsgid_test.go` among them, read Ze's messages through `decryptAndParse`. The engine function is deleted; `protected_input_test.go` declares a test reader of the same name that opens through `openProtected` under its own reason (`test-reader-unauthenticated`) and answers a drop as an error, so no test body changed and every assertion that a message is refused now runs the production entry. `TestDecryptMarksAuthenticatedPaddingFailureAsInner` runs unedited through it. `TestOneFunctionOpensEveryProtectedMessage` reads non-test files only, so the reader does not count. The one test edit is the `RFC7296-2.3-9 negative` claim of `TestImiUnauthenticatedRequestDrawsNothing`, which named `decryptAndParse` as what `handleOwnedInbound` calls and now names `openProtected`.
- Phase 3: an authenticated message that does not parse at the MOBIKE retransmit arm, the out-of-window INFORMATIONAL response arm, or the in-window site as a response, is still dropped with the SA kept, as before, and is now logged at WARN instead of debug or nothing: only the peer can seal it, so the line is not writable by an off-path sender, and `./le arch refuse-by-default` refuses an error dropped with only a debug line.
- Phase 4: the three WARN lines phase 3 added in `handleOwnedInbound` (an authenticated message that does not parse at the MOBIKE retransmit arm, the out-of-window INFORMATIONAL response arm and the in-window response site) are a second problem class: only the peer holds the keys, so the defect is the peer's implementation. The doctor-coverage gate refuses a WARN with no code a check declares, and the owner rule makes every problem class owe a doctor check, so phase 4 adds `ipsec-peer-malformed-message` (order 740, code `doctor-ipsec-peer-malformed-message`), counted on the SA (`SA.peerMalformedMessages`) at those three sites and reported while the SA lives, with `"code"` on each WARN line. A malformed in-window request is not counted: it draws INVALID_SYNTAX and ends the SA, as before.
- Phase 4: the drop count is taken on every drop, in every role and state (`dropUnauthenticated` calls `SA.countUnauthenticatedDrop`), and the check filters: only a handshake state is reported, and no peer that holds an established IKE SA (AC-11's survivor, read literally). The accessors live in `sa.go`, so `openProtected`'s own lines did not move and the hand-written mutant records keep their line numbers.
- Phase 6, two checker defects found on the way to green. First, the forgeries never reached Ze's IKE socket: charon had installed its 0/0 Child SA when it answered into the hold, and rawprobe's raw socket takes the XFRM output path, so they left as ESP under an SA Ze never installed (evidence: Ze `XfrmInNoStates` 0 to 20, UDP `InDatagrams` unchanged, the nft `forgeries` counter at 20). The fix is an outbound XFRM policy with no template on strongSwan for the address pair and proto udp, installed for the injection only. Second, that policy first carried ports and never matched, because `raw_sendmsg` (`net/ipv4/raw.c`) builds the flow of a non-HDRINCL raw socket with both ports 0; the selector names no port. The checker's assertions are numbered (the count is assertion 5, the initiator SPI assertion 7), so the red record cites the one that fails.
- AC-10 judges non-test files only, as `TestOneFunctionOpensEveryProtectedMessage` does: the phase 3 test reader named `decryptAndParse` stays in `protected_input_test.go`, because renaming it would edit about a hundred call sites, many in RFC-tagged test bodies. The Deliverables grep for `decryptAndParse` therefore excludes test files.
- The discrimination records of the `RFC7296-2.4-3` tags on the initiator use two hand-written mutants of `openProtected` rather than a revert of phase 2: one returns the drop as an error, the other drops an authenticated chain error. A revert replaces the entry with code that panics on the new tests' input, so both polarities go red together and the record proves nothing about which branch each test pins.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unprotected-message-changes-sa-state.md` | 2026-09-29 | ike initiator IKE_AUTH and EAP round: a response that fails its integrity check ends the SA |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| `unprotected-message-changes-sa-state.md` | 2026-08-14 | ike responder IKE_AUTH and EAP round | Fixed 2026-11-09 by 31f16fc038 and 7f6d281b6d: `handleAuthRequest`, `handleResponderEAP` and `handleResponderAuthRound` call `discardUnauthenticatedHandshakeRequest` (`ike/engine/responder_unauthenticated.go`). Proof: `rfc7296_unprotected_handshake_test.go`, interop `responder-eap-survives-unprotected-auth`. The journal row already reads `fixed:` |

## Rows considered and not owned

| Item | Why not here | Home |
|------|--------------|------|
| One transition function as the only writer of `sa.State` (about a hundred direct writes) | Owner decision D3, 2026-11-11: "Ok add it to the post-release spec" | `plan/post-release/engineering/spec-validated-construction-and-state-types.md` |
| `silent-fall-through.md`, 2026-11-10: an authenticated malformed IKE_AUTH request ends the responder's handshake with no notify (RFC 7296 Section 2.21.2) | A different obligation, on the branch this spec leaves as it is | `plan/release/fixes/spec-ike-responder-auth-malformed-request-notify.md` |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One entry that returns three outcomes and no error for the unauthenticated one | Call `discardUnauthenticatedHandshakeRequest` from the two initiator handlers as well | The second way fixes the two sites and leaves the bypass open: the next handler can call `decryptSKPayload` and act on its error again. It also keeps a helper whose name says "request" |
| The entry takes the reason from its caller | One reason for every drop | Two existing reasons are asserted by tests and tell an operator which guard fired; merging them changes a published series |
| The initiator shares `handshake-unauthenticated` | A reason of its own, such as one per role | The metric's help text and the guide define the label as the name of the guard, and the guard is now one function for both roles. The existing interop checker and three tests assert that exact series. The doctor check names the peer, whose connection type gives the role |
| New reason `established-unauthenticated` for the three sites that count nothing | Leave them uncounted | "Drop, count, log" is the entry's contract, and a drop nobody can see is the defect class the responder fix closed |
| Each site keeps its reaction to authenticated malformed content | Make the second exchange end on a CBC padding failure, as the first does | The fix is about unauthenticated input. No RFC 2119 sentence makes today's reaction wrong (Design Insights), so changing it would be a preference, and it would widen a release fix |
| A second doctor check, `ipsec-peer-malformed-message` (code `doctor-ipsec-peer-malformed-message`, order 740), counted on the SA where an established SA drops an authenticated message that does not parse, and where the RFC 4739 second exchange keeps the SA on a CBC padding failure | Leave those WARN lines without a code; fold them into `ipsec-unauthenticated-input` | `./le arch doctor-coverage check` refuses a WARN line with no code a check declares, and the owner rule makes every problem class an operator can hit owe a doctor check. The class differs from the forgery class: only the peer holds the keys, so the defect is the peer's implementation, and the fix line differs |
| The doctor's count lives on the SA of the current handshake | A per-peer counter beside the cookie counters; reading the global series | The SA's lifetime is the handshake's, so the warning clears with no reset logic; a cumulative series would warn forever after one forgery |
| The `.ci` injects through a relay fixture | A scripted IKE responder inside the fixture; a test-only hook in the engine | A relay needs no IKE implementation and no product code for tests |
| `TestOneFunctionOpensEveryProtectedMessage` reads the package source | A source gate under `le arch`; no test | Owner decision D1 rules out a gate. Go privacy cannot stop a same-package call, and the test is local to the package it guards |

## Known Limitations
- `sa.State` is still written directly at about a hundred sites. The single transition function is planned in `plan/post-release/engineering/spec-validated-construction-and-state-types.md` (owner decision D3).
- An authenticated malformed IKE_AUTH request still ends the responder's handshake without the notify RFC 7296 Section 2.21.2 owes: `plan/release/fixes/spec-ike-responder-auth-malformed-request-notify.md`.
- IKE_SA_INIT is unprotected by construction and is not touched here. Its handling (`handleSAInitResponse`, the COOKIE and INVALID_KE_PAYLOAD retries) has its own requirement rows.
- The drop is visible at debug level, in the counter and in `ze doctor` only. That is deliberate (R-8).

## RFC Documentation (Scope: protocol)

Above the drop in the entry: the whole Section 2.4 sentence quoted in Task,
byte-exact, with `RFC 7296 Section 2.4`, and the Section 2.4 rate-limit
sentence the existing helper carries. At each of the twelve call sites:
`// RFC 7296 Section 2.4`. The comments that cite Section 2.21.4 for a
response with no SK payload keep their quote where the missing-SK case is
decided, which is now the entry.

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
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path ./internal/component/ike/engine/`, `./le test unit pkg path ./internal/le/interoplab/ipsec/`, `./le test unit pkg path ./internal/test/fixture/`, the one `.ci` in `./le test functional ipsec`, and the one scenario in `./le test integration interop-ipsec`. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

# Spec: RSVP ResvErr, ResvTear and per-descriptor error reporting (RFC 2205)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-08 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The started implementation now builds and dispatches ResvErr and ResvTear, processes explicit flow descriptors independently and preserves an existing reservation when an attempted increase fails. The implementation is in `internal/plugins/rsvpte/reservation.go`, with native label/forwarding removal preceding reservation-state removal. Main must verify receiver-directed errors, mismatched tears, failed increases and forwarding cleanup before closure. General multicast and wildcard reservation support remain backlog.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC2205-2-8 | "Since a request that fails may be the result of merging a number of requests, a reservation error must be reported to all of the responsible receivers." (§2) | `reservation.go::handleResvErr` handles the implemented explicit-descriptor path; general multicast merge/fan-out is not claimed |
| RFC2205-3-13 | "A PathTear message may include a SENDER_TSPEC or ADSPEC object in its sender descriptor, but these must be ignored." (§3) | `engine.go::handlePathTear` removes matched state without using those optional objects; the quotation is a PathTear rule, not a ResvTear requirement |
| RFC2205-3-17 | "A ResvTear message must be routed like the corresponding Resv message, and its IP destination address will be the unicast address of a previous hop." (§3) | `reservation.go::handleResvTear` and `removeReservation` match downstream reservation state and relay toward the stored previous hop |
| RFC2205-3-18 | "Each flow descriptor in a FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error." (§3) | `reservation.go::acceptReservation` admits each descriptor independently; failed descriptors produce separate errors without removing accepted siblings |
| RFC2205-3-19 | "This ResvErr message must contain the information required to define the error and to route the error message in later hops." (§3) | The reservation error builder retains the session, error, style and failing descriptor for downstream processing |
| RFC2205-3-20 | "If the error is an admission control failure while attempting to increase an existing reservation, then the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC of the ResvErr message." (§3) | `reservation.go::acceptReservation` retains existing reservation/forwarding state on failed admission or installation and reports InPlace when an old reservation exists |

## Required Reading

| Source | Why |
|--------|-----|
| `rfc/full/rfc2205.txt` Sections 2.5, 3.1.5, 3.1.6, 3.1.8 | The six sentences the rows quote |
| `docs/contributing/rfc-conformance-gates.md` "The discrimination record" | Tag form, record fields, the revert route |
| `docs/architecture/rsvpte/mpls-rsvp-te.md` | Reservation control messages |

## Current Behavior

- [ ] `internal/plugins/rsvpte/reservation.go` -- `acceptReservation`, `handleResvErr`, `handleResvTear`, `removeReservation`
- [ ] `internal/plugins/rsvpte/reservation_build.go` -- `buildReservationControl`, `sendResvError`, `rejectReservation` (InPlace)
- [ ] `internal/plugins/rsvpte/engine.go` -- `handleResv` (per-descriptor loop), `handlePathTear`
- [ ] `internal/plugins/rsvpte/reroute.go` -- `pathTearLocked`, the relayed PathTear built from stored path state
- [ ] `internal/plugins/rsvpte/wire.go` -- `DecodeMessage` skips a PathTear's SENDER_TSPEC and ADSPEC

The producers were committed before this work; it adds proof only.

## Data Flow

### Entry Point
An RSVP Resv, ResvErr, ResvTear or PathTear packet reaching `engine.handlePacket`.

### Transformation Path
`DecodeMessage` -> `handleResv` / `handleResvErr` / `handleResvTear` / `handlePathTear` -> admission and FIB state -> `buildReservationControl` or `pathTearLocked` -> `Transport.Send` / `SendPath`.

### Boundaries Crossed
Wire bytes in, wire bytes out through the transport; admission controller and FIB inside the plugin.

### Integration Points
`transport.Send` (ResvErr, ResvTear), `transport.SendPath` (PathTear relay), `admissionController.reserve`, `fib.removeSwap`.

## Wiring Test

| Entry Point | -> Feature Code | -> Test |
|-------------|-----------------|---------|
| `engine.handlePacket` with encoded RSVP bytes | -> `reservation.go`, `reservation_build.go`, `engine.go::handlePathTear` | -> `rfc2205_resv_error_test.go`, decoding what the fake transport sent |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Requirement | Polarity |
|------|-------------|----------|
| `TestRFC2205ResvErrRelayedToReceiver` | RFC2205-2-8 | positive |
| `TestRFC2205ResvErrForOtherSenderNotRelayed` | RFC2205-2-8 | negative |
| `TestRFC2205PathTearObjectsIgnored` | RFC2205-3-13 | positive |
| `TestRFC2205PathTearTSpecNotUsed` | RFC2205-3-13 | negative |
| `TestRFC2205ResvTearRoutedLikeResv` | RFC2205-3-17 | positive |
| `TestRFC2205ResvTearForOtherHopNotRouted` | RFC2205-3-17 | negative |
| `TestRFC2205FixedFilterErrorPerDescriptor` | RFC2205-3-18 | positive |
| `TestRFC2205FixedFilterGoodDescriptorKept` | RFC2205-3-18 | negative |
| `TestRFC2205ResvErrCarriesErrorAndRoute` | RFC2205-3-19 | positive |
| `TestRFC2205ResvErrWrongStyleNotRouted` | RFC2205-3-19 | negative |
| `TestRFC2205FailedIncreaseLeavesReservationInPlace` | RFC2205-3-20 | positive |
| `TestRFC2205FailedFirstReservationNotInPlace` | RFC2205-3-20 | negative |

## Files to Modify

| File | Change |
|------|--------|
| `internal/plugins/rsvpte/rfc2205_resv_error_test.go` | New: the twelve tagged tests |
| `rfc/discrimination/rfc2205.json` | Twelve revert-route records |
| `rfc/short/rfc2205.md` | Six rows lose `{gap}`; Support remaining recounted |

## Implementation Steps

1. Write each test against the producer, driven through `handlePacket`.
2. Record each tag with `./le rfc discriminate-record` (revert route), which observes the red.
3. Prove the key semantics by Go overlay mutation (InPlace forced on and off, STYLE match removed).
4. Remove the six `{gap}` annotations.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL under a break of the producer (the discrimination records)
- [ ] Tests PASS
- [ ] `./le verify worktree` (owed by the main thread)

### Integration Checklist

- [ ] `./le rfc check` reports no stale or owed record for RFC 2205

### Documentation Update Checklist

- [ ] No page describes behavior this work changed: no producer code changed

## Risks & Assumptions

| Item | Note |
|------|------|
| Scope | Explicit-descriptor unicast reservations only; multicast merging and WF style remain absent features |
| Revert route | A revert record proves the unit reaches the producer; the overlay mutations above prove the semantic assertions |

## Acceptance Criteria

Evidence: `internal/plugins/rsvpte/rfc2205_resv_error_test.go`. Each test is
tagged `RFC requirement:` and carries a revert-route record in
`rfc/discrimination/rfc2205.json`: 12 records, a positive and a negative per row.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (RFC2205-2-8) | A transit holding a reservation receives a ResvErr for it from its previous hop; separately, a ResvErr naming a sender it holds no reservation for | The ResvErr is relayed to the next hop the Resv came from, ERROR_SPEC unchanged (`TestRFC2205ResvErrRelayedToReceiver`); the unrelated one reaches nobody (`TestRFC2205ResvErrForOtherSenderNotRelayed`) |
| AC-2 (RFC2205-3-13) | A PathTear carrying a SENDER_TSPEC and an ADSPEC with invalid IntServ bodies; separately, a PathTear whose SENDER_TSPEC differs from the PATH's | Path state is deleted, the tear is relayed, no PathErr (`TestRFC2205PathTearObjectsIgnored`); no SENDER_TSPEC is decoded from the tear and the relayed tear carries the stored TSPEC (`TestRFC2205PathTearTSpecNotUsed`) |
| AC-3 (RFC2205-3-17) | A ResvTear matching the transit's reservation; separately, one whose RSVP_HOP LIH does not match | The tear goes to the same unicast previous hop the Resv went to and the RSB is removed (`TestRFC2205ResvTearRoutedLikeResv`); the mismatched tear goes nowhere and the RSB stays (`TestRFC2205ResvTearForOtherHopNotRouted`) |
| AC-4 (RFC2205-3-18) | An FF Resv with one known and two unknown senders | Two separate ResvErr, one FILTER_SPEC each (`TestRFC2205FixedFilterErrorPerDescriptor`); the known sender is reserved with its own label and relayed, and no ResvErr names it (`TestRFC2205FixedFilterGoodDescriptorKept`) |
| AC-5 (RFC2205-3-19) | A Resv for an unknown sender; separately, a ResvErr whose STYLE does not match the reservation | The originated ResvErr carries SESSION, RSVP_HOP, ERROR_SPEC, the STYLE copy and the failing FLOWSPEC and FILTER_SPEC (`TestRFC2205ResvErrCarriesErrorAndRoute`); the mismatched ResvErr is not routed (`TestRFC2205ResvErrWrongStyleNotRouted`) |
| AC-6 (RFC2205-3-20) | A Resv raising an admitted reservation past interface capacity; separately, a first Resv past capacity | The old RSB, label, state and admission charge stay, nothing is relayed, and the ResvErr has code 1, value 2, InPlace on (`TestRFC2205FailedIncreaseLeavesReservationInPlace`); with nothing in place InPlace is off and no RSB exists (`TestRFC2205FailedFirstReservationNotInPlace`) |
| AC-7 | `rfc/short/rfc2205.md` | The six rows carry no `{gap}`; the Support remaining cell counts 43 MUST gaps |

Semantic discrimination beyond the revert records, by Go overlay on 2026-10-08:
forcing `inPlace` on reddens `TestRFC2205FailedFirstReservationNotInPlace`,
forcing it off reddens `TestRFC2205FailedIncreaseLeavesReservationInPlace`, and
dropping the STYLE match in `handleResvErr` reddens
`TestRFC2205ResvErrWrongStyleNotRouted`. No producer defect was exposed and no
producer code changed.

## Closure Review (2026-10-08): closure withheld on interop

Independent review of 6ad44e4ee0 by a session that wrote none of it.

| Check | Result |
|-------|--------|
| Claim against assertion, 12 tagged tests | Each `RFC requirement:` claim states what its test body asserts and no more |
| RFC quotes | All six sentences found verbatim in `rfc/full/rfc2205.txt` (Sections 2.5, 3.1.5, 3.1.6, 3.1.8) |
| `./le rfc check` | No violation names rfc2205 or rfc3209; its 164 violations are in BGP stems other sessions own |
| Scoped tests | `go test ./internal/plugins/rsvpte/` green, `go vet` clean |

**Defect fixed in review (RFC2205-3-13).** `DecodeMessage` (`wire.go`) ran
`knownCType` before the PathTear skip, so a PathTear whose SENDER_TSPEC or
ADSPEC carried an unknown C-Type was rejected whole and its path state stayed.
RFC 2205 Section 3.1.5 governs: "A PathTear message may include a SENDER_TSPEC
or ADSPEC object in its sender descriptor, but these must be ignored." Section
3.10 is a SHOULD stated generally: "Generally, the appearance of an object with
unknown C-Type should result in rejection of the entire message and generation
of an error message (ResvErr or PathErr as appropriate)." The specific MUST
wins, and an ignored object's C-Type is not examined. The skip now runs before
the C-Type check (`message_validation.go::pathTearIgnored`); the two dead
in-switch skips were removed. `TestPathTearUnknownCTypeObjectsIgnored` was red
before the fix (path state kept, no tear relayed) and is green after.
`docs/architecture/rsvpte/mpls-rsvp-te.md` states the rule.

**Interop gap: this spec does not close.** `ai/rules/interop-and-goal-validation.md`
requires a scenario against another implementation. The only RSVP-TE carrier,
`freertr_interop_integration_linux_test.go::TestRSVPFreeRouterInterop`, makes
Ze the egress of one freeRouter LSP: it exercises a peer PathTear carrying
ADSPEC (part of RFC2205-3-13) and nothing else this spec proves. No ResvErr,
ResvTear, FF multi-descriptor Resv or admission failure crosses the wire to an
independent peer, and Ze is never a transit. Scenario needed: freeRouter
ingress, Ze transit, freeRouter (or a second peer) egress, asserting at the
peers that (1) a ResvErr Ze originates for an over-capacity increase carries
InPlace and the old reservation keeps forwarding, (2) a peer ResvErr is relayed
to the egress, (3) a peer ResvTear reaches the ingress and removes Ze's swap
route, (4) an FF Resv with one unknown sender yields one ResvErr while the good
LSP stays up. The carrier needs root, so it was not run in this review.

## Interop status (2026-10-08): ResvErr relay proven, ResvTear red on freeRouter's parser

`./le test integration interop-rsvpte` (`internal/le/interoplab/rsvpte/`,
catalog suite `rsvpte`) ran on 2026-10-08 over a tree carrying the MPLS
path-MTU probe (f3caa99352), so a Ze transit installs its swap and the LSP comes
up. The suite now has back-to-back scenarios: Ze originates, an unpatched
freeRouter is the independent implementation that parses the message, keeps the
state it needs and re-encodes it, and a second Ze captures what freeRouter sent.
freeRouter relays what Ze originates; it neither originates ResvErr, ResvTear or
PathErr nor enforces strict hops itself (`rtrRsvpIface.recvPack`), so the
evidence is that freeRouter accepts and relays Ze's messages. Run log: session
scratch `rsvpte-run8.log` (3 passed, 2 failed). Discrimination:
`rsvpte-disc-run10.log`, a copy of the tree with the producer broken, run with
`ze_repo_root=<copy>`.

| Needed (Closure Review) | Scenario | Result |
|-------------------------|----------|--------|
| (1) Ze ResvErr with InPlace on an increase | none | Not covered: freeRouter re-signals a new LSP-ID on a bandwidth change (`clntMplsTeP2p.workDoer`), so no peer drives an in-place increase |
| (2) a ResvErr relayed across a peer | `ingress-resv-error-relayed` | PASS. The Ze ingress refuses the RESV freeRouter relays (admission), freeRouter captures its ResvErr, and the Ze egress captures it as freeRouter relays it, error node, code 1 and value 2 intact. Discrimination: with the admission refusal in `acceptReservation` sending Traffic Control instead, the check goes red ("relayed ResvErr carries Error Code 22, want 1 Admission Control failure"); restored, it passes |
| (3) a ResvTear reaching the ingress | `transit-resv-tear-relayed` | FAIL. With the Ze egress frozen (`SIGSTOP`), the Ze transit's reservation times out and its ResvTear is in freeRouter's capture, but freeRouter never relays it: `packRsvp.parseDatResTer` refuses a ResvTear without a FLOWSPEC, and Ze omits it (`buildReservationControl`). RFC 2205 Section 3.1.6: "FLOWSPEC objects in the flow descriptor list of a ResvTear message will be ignored and may be omitted." |
| (4) FF Resv with an unknown sender | none | Not covered: every freeRouter RESV names its own one sender |

Not ready for closure. The owner decides how (3) is carried (Ze sends the
optional FLOWSPEC in a ResvTear, a freeRouter patched in the image, or another
implementation) and how (1) and (4) are proven.

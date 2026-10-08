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

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Twelve tagged tests, a positive and a negative for each of the six rows | `grep -c "RFC requirement: RFC2205-" internal/plugins/rsvpte/rfc2205_resv_error_test.go` prints 12 |
| Tests pass | `./le job run label rsvpte-rfc2205 command go test -tags ze_rsvpte -run 'TestRFC2205\|TestPathTearUnknownCTypeObjectsIgnored' ./internal/plugins/rsvpte/` (ok on 2026-10-08) |
| Twelve discrimination records for those tests | `rfc/discrimination/rfc2205.json` holds 12 records whose `unit` is in `rfc2205_resv_error_test.go` for RFC2205-2-8, 3-13, 3-17, 3-18, 3-19, 3-20 |
| No stale or owed RFC 2205 record or verdict | `./le rfc check` prints no line naming `rfc2205` (true after f3f77ffbf2 re-recorded the ten records and four verdicts that 1d457b0d52 and f9180d62a6 staled) |
| The six rows carry no `{gap}`; Support remaining counts 43 MUST gaps | `grep -E "\[RFC2205-(2-8\|3-13\|3-17\|3-18\|3-19\|3-20)\]" rfc/short/rfc2205.md \| grep -c "{gap"` prints 0; `grep -E "^- \[ \] \[RFC2205-[^]]+\] \[MUST" rfc/short/rfc2205.md \| grep -c "{gap"` prints 43 |
| PathTear unknown C-Type fix from review (RFC2205-3-13) | `TestPathTearUnknownCTypeObjectsIgnored` passes; `message_validation.go::pathTearIgnored` runs before `knownCType` in `wire.go::DecodeMessage` |
| Interop: the four Closure Review items | `./le test integration interop-rsvpte` scenarios `transit-resv-increase-refused-in-place`, `ingress-resv-error-relayed`, `transit-resv-tear-relayed`, `transit-ff-resv-unknown-sender` pass (needs root; logs named in "Interop status" below) |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | ResvErr and ResvTear come from any RSVP peer. `handleResvErr` acts only on a held reservation whose STYLE matches and whose previous hop is the source (`samePeer`); `handleResvTear` also requires the RSVP_HOP LIH and the next hop to match. The STYLE mismatch (AC-5), the unknown sender (AC-1) and the LIH mismatch (AC-3) are proven by negative tests. A wrong source address is proven by `internal/plugins/rsvpte/resv_control_source_test.go`: `TestResvErrFromWrongSourceIgnored` and `TestResvTearFromWrongSourceIgnored` send a fully matching ResvErr and ResvTear from a stranger and from the neighbour on the wrong side, and assert no message is forwarded, the swap entry stays, and the reservation, blockade, labels and state are unchanged; `TestResvErrFromPreviousHopActedOn` and `TestResvTearFromNextHopActedOn` are the positive controls from the right hop. Removing either `samePeer` call turns its negative test red (recorded 2026-10-08). The tests are untagged: RFC 2205 Sections 3.1.6 and 3.1.8 state where these messages are sent, not that a receiver refuses one from another source, so no rfc2205 row names the check |
| Ignored objects | A PathTear's SENDER_TSPEC and ADSPEC are skipped before any decode or C-Type check, so a malformed body in either cannot reject the tear (AC-2, `TestPathTearUnknownCTypeObjectsIgnored`) |
| Resource exhaustion | An FF Resv yields at most one ResvErr per failing descriptor, bounded by the descriptors one message can carry (message Length is 16 bits); no per-descriptor goroutine or unbounded state. A failed increase keeps the old admission charge (AC-6), and a failed install releases the label it allocated (`acceptReservation`, the `allocated` branch), so repeated over-capacity Resvs leak neither bandwidth nor labels |
| Fail-open authorization | A failed admission or install leaves the existing reservation and forwarding in place and never installs the refused one (AC-6); a first reservation that fails leaves no RSB (AC-6 negative) |
| Error leakage | A ResvErr carries only SESSION, RSVP_HOP, ERROR_SPEC, STYLE and the failing descriptor, all values the peer already sent or the RFC requires (AC-5) |

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

## Interop status (2026-10-08): ResvErr relay and ResvTear relay proven

`./le test integration interop-rsvpte` (`internal/le/interoplab/rsvpte/`,
catalog suite `rsvpte`) ran on 2026-10-08 over a tree carrying the MPLS
path-MTU probe (f3caa99352), so a Ze transit installs its swap and the LSP comes
up. The suite now has back-to-back scenarios: Ze originates, an unpatched
freeRouter is the independent implementation that parses the message, keeps the
state it needs and re-encodes it, and a second Ze captures what freeRouter sent.
freeRouter relays what Ze originates; it neither originates ResvErr, ResvTear or
PathErr nor enforces strict hops itself (`rtrRsvpIface.recvPack`), so the
evidence is that freeRouter accepts and relays Ze's messages. Run log: session
scratch `rsvpte-run8.log` (3 passed, 2 failed) before commit 9b8bfe250c, and
`rsvpte-run9.log` (5 passed, 0 failed) after it. Discrimination:
`rsvpte-disc-run10.log`, a copy of the tree with the producer broken, run with
`ze_repo_root=<copy>`.

| Needed (Closure Review) | Scenario | Result |
|-------------------------|----------|--------|
| (1) Ze ResvErr with InPlace on an increase | `transit-resv-increase-refused-in-place` | PASS (`rsvpte-inplace-run4.log`). Patched freeRouter egress (see below): the ingress signals 10 Mbit/s, Ze's eth0 reserves 100 Mbit/s, and the egress's second RESV raises the same session, sender and LSP-ID to 1 Gbit/s. The egress captures Ze's ResvErr naming the transit, code 1 value 2, ERROR_SPEC flags `0x01`; the swap stays installed and the ingress never receives the raised rate. Discrimination (`rsvpte-inplace-disc3.log`): with `old != nil` replaced by `false` in the admission-failure `rejectReservation` call of `acceptReservation`, the check goes red ("ResvErr for a failed increase does not carry InPlace (0x01)", captured `Flags: [0x00]`); restored, it passes |
| (2) a ResvErr relayed across a peer | `ingress-resv-error-relayed` | PASS. The Ze ingress refuses the RESV freeRouter relays (admission), freeRouter captures its ResvErr, and the Ze egress captures it as freeRouter relays it, error node, code 1 and value 2 intact. Discrimination: with the admission refusal in `acceptReservation` sending Traffic Control instead, the check goes red ("relayed ResvErr carries Error Code 22, want 1 Admission Control failure"); restored, it passes |
| (3) a ResvTear reaching the ingress | `transit-resv-tear-relayed` | PASS (`rsvpte-run9.log`). With the Ze egress frozen (`SIGSTOP`), the Ze transit's reservation times out, its ResvTear is in freeRouter's capture, and the Ze ingress captures it as freeRouter relays it. Red in `rsvpte-run8.log` while Ze omitted the optional FLOWSPEC, which `packRsvp.parseDatResTer` requires (RFC 2205 Section 3.1.6: "FLOWSPEC objects in the flow descriptor list of a ResvTear message will be ignored and may be omitted."). Since 9b8bfe250c `buildReservationControl` writes the torn-down reservation's FLOWSPEC, per the owner's decision of 2026-10-08 |
| (4) FF Resv with an unknown sender | `transit-ff-resv-unknown-sender` | PASS (`rsvpte-ff-run1.log`). Patched freeRouter egress: every RESV is fixed-filter with a second descriptor naming LSP-ID+1 of the same sender, which has no PATH. The egress captures one ResvErr from the transit naming only the unknown LSP-ID, Error Code 4 No sender information, and none naming the real sender; the ingress receives a labeled FF RESV naming only the real sender and the swap is installed. Discrimination (`rsvpte-ff-disc.log`): with the `!found` branch of `acceptReservation` returning before `rejectReservation`, the check goes red ("egress receives the Ze transit's ResvErr" timed out, with the two-descriptor FF RESV in the egress's capture); restored, it passes |

How (1) and (4) are driven (owner decision 2026-10-08: patch freeRouter, no
separate spec): the pinned freeRouter can neither raise a reservation in place
nor name a sender it has no path for, so `Dockerfile.freertr` applies
`test/interop-rsvpte/freertr/ze-interop-resv.patch` to the pinned revision. It
adds two knobs to the RESV a freeRouter egress originates, each off unless its
environment variable is set, which a scenario sets in `egress-env.txt`
(`docs/architecture/testing/interop.md`). The five earlier scenarios set
neither and run the upstream behavior. RFC 2205 Section 3.1.8: "If the error is
an admission control failure while attempting to increase an existing
reservation, then the existing reservation must be left in place and the
InPlace flag bit must be on in the ERROR_SPEC of the ResvErr message." and
"Each flow descriptor in a FF-style Resv message must be processed
independently, and a separate ResvErr message must be generated for each one
that is in error." Appendix B, Error Code 04: "There is path state for this
session, but it does not include the sender matching some flow descriptor
contained in the Resv message." No Ze defect was found on either item. Logs are
in session scratch `tmp/session/2026-10-07-450bc92b-6ac1-4190-bd40-b427ecba17bf/scratch/`;
the red runs used a copy of the tree with the producer broken, run with
`ZE_REPO_ROOT=<copy>`. Full suite after the change: `rsvpte-full-run.log`.

All four Closure Review interop items now have a passing scenario. Closure
itself stays with the spec's close phase.

# IKE/EAP AC-C2 record sweep, 2026-10-02

Scope: every verdict in the ike-eap child's owned stems (rfc2759, rfc3748, rfc3948, rfc4301,
rfc4303, rfc4555, rfc5216, rfc5282, rfc7296, rfc7427) that is `enforced` now and was not
`enforced` at `52749928c1~1` (the tree before the child's first commit, `52749928c1`), plus
RFC9190-5.10-1 (rfc9190 has no audit file; its 5.10-1 records were named in the brief).
136 ids: rfc2759 8, rfc3748 22, rfc3948 4, rfc4301 31, rfc4303 4, rfc4555 2, rfc5216 16,
rfc5282 8, rfc7296 40, rfc7427 1. For each stem, `./le rfc discriminate stem <stem> | json`
was read for `unproven` (a tagged unit with no record for its current claim) and `stale`.

- rfc4301, rfc4303, rfc5282: every tagged unit of the 43 ids carries a current record.
- 123 unproven units over rfc2759 15, rfc3748 22, rfc3948 4, rfc4555 5, rfc5216 5,
  rfc7296 61, rfc7427 3, RFC9190-5.10-1 8.
- Stale records in scope: 12. The 8 RFC9190-5.10-1 records on `peer.go::startTLSClient`
  (producer changed; the same 8 units are also unproven because their claims changed, so one
  record each clears both), and 4 ORPHANS (below).

Script: `tmp/session/2026-10-02-e08980d7-b739-4095-ab66-53539559487d/scratch/ike-ac-c2-sweep.sh`
(revert route, per-stem flock in `tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch/children/ledger-<stem>.lock`,
producers tried in order: the producers of the id's existing same-polarity records first,
then the other polarity's; first observed red stops). Log: `.../scratch/ike-ac-c2-sweep.tsv`,
one log per attempt under `.../scratch/ikesweeplogs/`. Plan: `.../scratch/ikec2/plan2.tsv`.

## Orphans (owner decision; not deleted, JSON not edited)

| id | polarity | unit | producer | result |
|----|----------|------|----------|--------|
| RFC3748-5.4-1 | negative | internal/core/eap/rfc3748_md5challenge_test.go::TestRFC3748MD5ChallengeRequeryDrawsNoResponse | internal/core/eap/peer.go::handleRequest | orphan: unit carries no 5.4-1 negative tag; it is on rfc3748_clause_test.go::TestRFC3748MD5ChallengeAnswersOnlyARequestThatCarriesOne, recorded on peer.go::handleMD5ChallengeRequest |
| RFC3748-5.4-2 | negative | internal/core/eap/rfc3748_md5challenge_test.go::TestRFC3748MD5ChallengeIsTheConfiguredMethod | internal/core/eap/peer.go::naks | orphan: tag now on rfc3748_clause_test.go::TestRFC3748MD5ChallengeServerRefusesAWrongValue, recorded on eap_md5challenge.go::Process |
| RFC4301-4.4.2.1-1 | positive | internal/component/ike/dataplane/rfc4301_boundary_linux_test.go::TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp | internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams | orphan: the file carries no 4.4.2.1-1 tag; the tags are in rfc4301_sad_items_linux_test.go, whose units discriminate reports as proven |
| RFC4301-4.4.2.1-1 | negative | internal/component/ike/dataplane/rfc4301_boundary_linux_test.go::TestRFC4301BoundarySADEntryIsRefusedRatherThanInstalledWrong | internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams | orphan: as above |

Out of scope, noted: RFC9190-2.1.9-2 has 2 stale records on `eap_tls.go::reassemble`
(rfc9190 is un-enrolled, no verdict), and rfc9190 holds 11 more unproven units outside 5.10-1.

## Results

Counts: 123 units owed (the 8 RFC9190-5.10-1 units count once; one record clears both their
stale and unproven state). 120 OBSERVED reds written, one per unit, 57 ids: rfc2759 15,
rfc3748 22, rfc3948 3, rfc4555 4, rfc5216 5, rfc7296 60, rfc7427 3, RFC9190-5.10-1 8 (all 8
on `internal/core/eap/peer.go::startTLSClient`). After the sweep, `./le rfc discriminate stem`
over the ten stems plus rfc9190 shows no stale record in scope except the 4 orphans, and the 3
unproven units below. Every recorded row is in `ike-ac-c2-sweep.tsv` with `rc=0` (first red
wins; earlier `rc=2` rows on the same unit are refusals, almost all R-10 "never executes" for a
producer in another package, plus 9 attempts that used a `(*T).M` producer spelling the tool
does not accept). The parent session restart killed one loop mid-run; it was resumed from the
TSV and nothing was recorded twice.

Producer choices a judge should look at:
- RFC7296-3.3-7 (wire/rfc7296_sa_test.go::TestPropAlternateKeyLengthsUseSeparateTransforms):
  the obligation's producer is `(*Transform).ReadFrom`, but `payload_sa.go` declares three
  `ReadFrom` methods and the revert route refuses an ambiguous name. Recorded instead on
  `payload_sa.go::checkAttr` (positive, the per-attribute check inside Transform.ReadFrom) and
  `payload_sa.go::rejectTransform` (negative, the refusal ErrDuplicateAttr goes through).
- RFC7296-3.6-2 positive `test/parse/ipsec-hash-and-url-accepted.ci`: recorded on
  `ipsec/config_auth_policy.go::parseCertificatePolicy` with citation
  `expect=stdout:contains=configuration valid` (a functional record needs one).
- RFC3748-4-1 (core/eap rfc3748_test.go) recorded on `eap.go::DecodePacket`; the id's other
  records sit on `engine/eap_auth.go::eapMessageDiscarded`, which this package never reaches.

No red (3):

| id | polarity | unit | producer | result |
|----|----------|------|----------|--------|
| RFC3948-4-1 | positive | internal/component/ike/transport/keepalive_test.go::TestKeepaliveDefaultInterval | tried engine/established.go::startNATKeepalive, engine/mobike.go::serviceMobike, transport/keepalive.go::Run, transport/keepalive.go::NewKeepalive | FINDING: the body asserts only the constant `DefaultKeepaliveInterval`; no function produces it, so no revert can break it (each producer refused R-10, never executed). Needs a test that reaches the keepalive timer, or an escape decided by a judge |
| RFC7296-3.1-3 | negative | internal/component/ike/engine/rfc7296_header_test.go::TestSPIZeroRules | tried engine/register.go::dispatchInbound, engine/register.go::dispatchNATTInbound | FINDING: the claim says dispatchInbound drops a datagram whose initiator SPI is zero, but the body never calls it (R-10 refusal on both). The tag claims more than the unit asserts |
| RFC4555-4.2.5-1 | positive | internal/component/ike/engine/mobike_test.go::TestMobikeAdvertisementAfterSourceChange | engine/mobike.go::startMobikeRequest | NOT RUN: the test `t.Skip`s off Linux (IP_PKTINFO), so on darwin the producer is never executed; with `kernel <vmlinuz>` the tool refuses ("runs on this host, so `kernel` names a guest nothing would boot") because the file has no linux build tag. Owed on a Linux host: `flock <children>/ledger-rfc4555.lock ./le rfc discriminate-record id RFC4555-4.2.5-1 polarity positive unit internal/component/ike/engine/mobike_test.go::TestMobikeAdvertisementAfterSourceChange route revert producer internal/component/ike/engine/mobike.go::startMobikeRequest` |

Files changed by this sweep: `rfc/discrimination/rfc2759.json`, `rfc3748.json`,
`rfc3948.json`, `rfc4555.json`, `rfc5216.json`, `rfc7296.json`, `rfc7427.json`,
`rfc9190.json` (records only, all written by `./le rfc discriminate-record`), and this file.
No test, code, audit or row was edited, nothing stamped or committed.
Gates owed by the main thread: `./le rfc check` over these stems.

## Judge (independent, 2026-10-02)

Scope re-derived from `rfc/audit/<stem>.json` against `52749928c1~1`: 136 ids newly `enforced`
over the ten stems (rfc2759 8, rfc3748 22, rfc3948 4, rfc4301 31, rfc4303 4, rfc4555 2,
rfc5216 16, rfc5282 8, rfc7296 40, rfc7427 1), plus RFC9190-5.10-1. It matches the sweep's set.

Records: the working tree adds exactly 120 records over HEAD (rfc2759 15, rfc3748 22, rfc3948 3,
rfc4555 4, rfc5216 5, rfc7296 60, rfc7427 3, rfc9190 8), all `route revert`, and removes only
the 8 stale RFC9190-5.10-1 records they replace. `./le rfc discriminate stem` over the eleven
stems, filtered to the in-scope ids, leaves only the 4 orphan stale records and the 3 unproven
units below. Every other unproven unit those stems print belongs to an id outside AC-C2.

Producer spot-checks:
- RFC7296-3.3-7, wire `TestPropAlternateKeyLengthsUseSeparateTransforms`: accepted. For the
  packed input (one ENCR transform, two TV Key Length attributes) both attributes pass
  `checkAttr`, and the duplicate loop in `(*Transform).ReadFrom` returns
  `rejectTransform(ErrDuplicateAttr)`, so with that input `rejectTransform` is reached on the
  duplicate refusal and nowhere else. The positive's `checkAttr` record proves reach of the
  per-attribute parse. Both records are reach-level rather than a break of the duplicate check
  itself. That is acceptable because the row's send obligation is proven separately by engine
  `TestRFC7296AlternateKeyLengthsAreOfferedAsSeparateTransforms` (recorded on
  `initiator.go::wireIKEOffer`), and the wire unit is the receive-side negative.
- RFC7296-2.20-1 negative, `rfc7296_cp_test.go::TestZeDeclinesApplicationVersion`: the sweep
  recorded it on `mobike.go::mobikeResponsePayloads`, which only the positive half (the build
  sweep) reaches. The negative half is a PayloadCP codec round trip, so that red was the
  positive half's panic, not the negative's. The judge re-recorded it on
  `internal/component/ike/wire/payload_cp.go::ReadFrom` (observed red at the test's
  `back.ReadFrom` call), which replaced the old record.
- Also read: RFC2759-x-12, RFC3748-2.1-4, RFC3748-4-1, RFC7296-1.2-1, 3.1-3 positive, 3.1-4,
  RFC7427-3-4, RFC9190-5.10-1. Each producer is the function the unit's assertion depends on.

Findings:
- RFC3948-4-1 positive `TestKeepaliveDefaultInterval`: the verdict does NOT rest on it.
  `DefaultKeepaliveInterval` reaches production (`runEstablished` in `established.go` passes it
  to `startNATKeepalive`; `NewKeepalive` falls back to it), and seven recorded units prove the
  clause: the need gate, both polarities on both paths (`rfc3948_keepalive_need_test.go`,
  4 units), the M-second window (`TestRFC3948KeepaliveIdleWindowNeverExceedsM`,
  `TestRFC3948UnsetKeepaliveIntervalFallsBackToM`), and `TestNATKeepalive` on
  `keepalive.go::Run`. The verdict stays `enforced`, with no re-stamp. The unit itself can
  never carry a record: reverting a function leaves a constant intact, a mutant inside the
  `const` block is refused ("sits in no function", `mutantBreak`), and `declaration-only` is
  refused because `keepalive.go` declares functions. OWED (author): either drop the tag (a D-15
  tag removal), since the fallback unit already proves the default, or rewrite the unit to
  drive `NewKeepalive` with an unset interval and assert the interval it runs at, then record
  it on `keepalive.go::NewKeepalive`. Either way, narrow the prose: "well under a typical NAT
  UDP binding lifetime, so keepalives refresh the mapping" is not asserted.
- RFC7296-3.1-3 negative `TestSPIZeroRules#2`: the claim ("dispatchInbound drops a datagram
  whose initiator SPI is all zeroes", with a register.go line range) overreaches. The body
  never calls `dispatchInbound`, and the cited line range is stale. The verdict does not rest
  on it: `rfc7296_zero_ispi_test.go::TestRFC7296ZeroInitiatorSPIIsDroppedOnReceipt` (port 500,
  runs `dispatchInbound` with a zero-iSPI SA in the table, asserts no delivery) and
  `rfc7296_zero_ispi_natt_test.go::TestRFC7296ZeroInitiatorSPIIsDroppedOnTheNATTSocket`
  (`dispatchNATTInbound`) are tagged negatives with records on those dispatchers. The verdict
  stays `enforced`, with no re-stamp. OWED (author): remove the `RFC7296-3.1-3 negative` tag
  from `TestSPIZeroRules` (D-15 removal; the two dedicated units carry the proof). The body
  asserts nothing about receipt, so a reworded tag would have nothing left to claim.
- RFC4555-4.2.5-1 positive `mobike_test.go::TestMobikeAdvertisementAfterSourceChange`: left
  owed. It `t.Skip`s off Linux, so its record needs a Linux host (command in the table above).
- Orphans (RFC3748-5.4-1/-2 negatives, the two RFC4301-4.4.2.1-1 records): untouched, for the
  owner.

`./le rfc check` (2026-10-02, after the re-record): 11 violations, and the only ones in this
child's stems are the 2 out-of-scope RFC9190-2.1.9-2 producer-changed records named above.
The other 9 are BGP stems (rfc9552 stale verdicts, rfc4271, rfc4456, rfc8277 records).

AC-C2 for this child: met for 120 of 123 units. 3 units stay owed: two need an author change
to the test or its tag, and one needs a Linux host. No verdict changed and no audit file was
stamped.
